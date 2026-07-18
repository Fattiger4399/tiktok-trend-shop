package apiv1

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"tiktok-trend-shop/internal/auth"
)

type contextKey string

const userContextKey contextKey = "apiv1.user"

// UserFromContext returns the authenticated user attached by requireAuth.
func UserFromContext(ctx context.Context) (*auth.User, bool) {
	user, ok := ctx.Value(userContextKey).(*auth.User)
	return user, ok
}

// requireAuth wraps the whole /api/v1 mux: every route except the login
// endpoint needs a valid "Authorization: Bearer <token>" header.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/login" {
			next.ServeHTTP(w, r)
			return
		}
		token, ok := bearerToken(r)
		if !ok {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}
		user, err := s.auth.Authenticate(r.Context(), token)
		if err != nil {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, &user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(r *http.Request) (string, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	token = strings.TrimSpace(token)
	return token, ok && token != ""
}

// operatorOnly restricts a handler to operator users; clients get a 403.
func (s *Server) operatorOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok || user.Role != auth.RoleOperator {
			WriteError(w, http.StatusForbidden, "forbidden", "operator role required")
			return
		}
		h(w, r)
	}
}

// clientOwnsRequest enforces client data isolation: client users may only
// touch material requests whose client_id matches their own user id.
func clientOwnsRequest(w http.ResponseWriter, r *http.Request, clientID string) bool {
	user, ok := UserFromContext(r.Context())
	if !ok || user.Role != auth.RoleClient {
		return true
	}
	if clientID != user.ID {
		WriteError(w, http.StatusForbidden, "forbidden", "request belongs to another client")
		return false
	}
	return true
}

// resolveActor picks the review actor: body values stay authoritative for
// operators (backwards compatible), otherwise the login name wins. Clients
// always act as themselves.
func resolveActor(r *http.Request, bodyActor string) string {
	user, ok := UserFromContext(r.Context())
	if !ok {
		return bodyActor
	}
	if user.Role == auth.RoleClient {
		return user.Username
	}
	if bodyActor != "" {
		return bodyActor
	}
	return user.Username
}

// requestAccessible reports whether the current user may see or act on the
// given material request: missing requests produce a 404 and requests owned
// by another client produce a 403.
func (s *Server) requestAccessible(w http.ResponseWriter, r *http.Request, id string) bool {
	req, err := s.request.Get(r.Context(), id)
	if err == sql.ErrNoRows {
		WriteNotFound(w, "material_request")
		return false
	}
	if err != nil {
		WriteInternal(w, err)
		return false
	}
	return clientOwnsRequest(w, r, req.ClientID)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		WriteFieldError(w, "body", "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.Username) == "" {
		WriteFieldError(w, "username", "username is required")
		return
	}
	if body.Password == "" {
		WriteFieldError(w, "password", "password is required")
		return
	}
	token, user, err := s.auth.Login(r.Context(), body.Username, body.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		WriteError(w, http.StatusUnauthorized, "invalid_credentials", err.Error())
		return
	}
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"token": token, "user": user})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if token, ok := bearerToken(r); ok {
		if err := s.auth.Logout(r.Context(), token); err != nil {
			WriteInternal(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	WriteJSON(w, http.StatusOK, user)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		Role        string `json:"role"`
		DisplayName string `json:"display_name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		WriteFieldError(w, "body", "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.Username) == "" {
		WriteFieldError(w, "username", "username is required")
		return
	}
	if body.Password == "" {
		WriteFieldError(w, "password", "password is required")
		return
	}
	if body.Role != auth.RoleOperator && body.Role != auth.RoleClient {
		WriteFieldError(w, "role", "role must be operator or client")
		return
	}
	user, err := s.auth.CreateUser(r.Context(), body.Username, body.Password, body.Role, body.DisplayName)
	if errors.Is(err, auth.ErrUsernameTaken) {
		WriteError(w, http.StatusConflict, "username_taken", err.Error())
		return
	}
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, user)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.auth.ListUsers(r.Context())
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, ListResponse{Items: users, Pagination: PaginationFor(1, MaxPageSize, len(users))})
}
