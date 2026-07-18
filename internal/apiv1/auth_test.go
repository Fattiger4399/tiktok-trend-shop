package apiv1_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tiktok-trend-shop/internal/auth"
	"tiktok-trend-shop/internal/request"
	"tiktok-trend-shop/internal/review"
)

func TestMissingOrInvalidTokenRejected(t *testing.T) {
	h := newHarness(t)

	// No Authorization header at all.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends", nil)
	rr := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d body=%s", rr.Code, rr.Body.String())
	}
	if env := errorEnvelopeOf(t, rr); env.Code != "unauthorized" {
		t.Fatalf("expected unauthorized envelope got %#v", env)
	}

	// A malformed token is also rejected.
	rr = h.doAs("not-a-real-token", http.MethodGet, "/api/v1/trends", "", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestLoginSuccessAndFailure(t *testing.T) {
	h := newHarness(t)
	jsonHeaders := map[string]string{"Content-Type": "application/json"}

	// Wrong password and unknown user both fail with a 401 envelope.
	for _, body := range []string{
		`{"username":"admin","password":"wrong"}`,
		`{"username":"ghost","password":"admin123"}`,
	} {
		rr := h.do(http.MethodPost, "/api/v1/auth/login", body, jsonHeaders)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 got %d body=%s", rr.Code, rr.Body.String())
		}
		if env := errorEnvelopeOf(t, rr); env.Code != "invalid_credentials" {
			t.Fatalf("expected invalid_credentials got %#v", env)
		}
	}

	// Correct credentials return a token and the user profile.
	rr := h.do(http.MethodPost, "/api/v1/auth/login",
		`{"username":"admin","password":"admin123"}`, jsonHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "password_hash") {
		t.Fatalf("login response leaks password hash: %s", rr.Body.String())
	}
	var loginResp struct {
		Token string    `json:"token"`
		User  auth.User `json:"user"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &loginResp); err != nil {
		t.Fatal(err)
	}
	if loginResp.Token == "" || loginResp.User.Username != "admin" ||
		loginResp.User.Role != auth.RoleOperator {
		t.Fatalf("unexpected login payload %#v", loginResp)
	}

	// The issued token authenticates /auth/me.
	rr = h.doAs(loginResp.Token, http.MethodGet, "/api/v1/auth/me", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var me auth.User
	if err := json.Unmarshal(rr.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if me.Username != "admin" || me.Role != auth.RoleOperator {
		t.Fatalf("unexpected me payload %#v", me)
	}

	// Logout invalidates the token.
	rr = h.doAs(loginResp.Token, http.MethodPost, "/api/v1/auth/logout", "", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.doAs(loginResp.Token, http.MethodGet, "/api/v1/auth/me", "", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestOperatorManagesUsers(t *testing.T) {
	h := newHarness(t)
	jsonHeaders := map[string]string{"Content-Type": "application/json"}

	rr := h.do(http.MethodPost, "/api/v1/users",
		`{"username":"client-a","password":"secret-a","role":"client","display_name":"甲方 A"}`, jsonHeaders)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d body=%s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "password_hash") {
		t.Fatalf("create user response leaks password hash: %s", rr.Body.String())
	}
	var created auth.User
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.ID, "usr_") || created.Role != auth.RoleClient ||
		created.DisplayName != "甲方 A" {
		t.Fatalf("unexpected user %#v", created)
	}

	// Duplicate usernames conflict.
	rr = h.do(http.MethodPost, "/api/v1/users",
		`{"username":"client-a","password":"secret-a","role":"client"}`, jsonHeaders)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 got %d body=%s", rr.Code, rr.Body.String())
	}

	// Unknown roles are rejected.
	rr = h.do(http.MethodPost, "/api/v1/users",
		`{"username":"nobody","password":"secret-a","role":"admin"}`, jsonHeaders)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.do(http.MethodGet, "/api/v1/users", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var list struct {
		Items []auth.User `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 users got %#v", list.Items)
	}
}

func TestClientCanAccessOperatorRoutes(t *testing.T) {
	// 单端模式：角色墙停用，client 与 operator 等价，原 403 用例改为
	// 断言 client 可访问全部路由。恢复 RBAC 时这些应重新断言 403。
	h := newHarness(t)
	h.mustUser("client-a", "secret-a", auth.RoleClient)
	clientToken := h.mustLogin("client-a", "secret-a")
	jsonHeaders := map[string]string{"Content-Type": "application/json"}

	rr := h.doAs(clientToken, http.MethodGet, "/api/v1/imports", "", jsonHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("imports: expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.doAs(clientToken, http.MethodGet, "/api/v1/users", "", jsonHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("users: expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.doAs(clientToken, http.MethodPost, "/api/v1/users",
		`{"username":"client-b","password":"secret-b","role":"client"}`, jsonHeaders)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create user: expected 201 got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.doAs(clientToken, http.MethodGet, "/api/v1/deliveries", "", jsonHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("deliveries: expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}

	// Unknown ids still surface the handler's own 404, not a role 403.
	rr = h.doAs(clientToken, http.MethodPost, "/api/v1/requests/req_any/generate", "", jsonHeaders)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("generate: expected 404 got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.doAs(clientToken, http.MethodPost, "/api/v1/requests/req_any/deliver",
		`{"variant_id":"cv_x"}`, jsonHeaders)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("deliver: expected 404 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestClientSeesAllRequests(t *testing.T) {
	// 单端模式：列表不再按 client_id 过滤，创建时 body 的 client_id 原样
	// 生效（可空），任何登录用户可读任何需求。
	h := newHarness(t)
	p := h.mustProduct("Lamp")
	clientA := h.mustUser("client-a", "secret-a", auth.RoleClient)
	h.mustUser("client-b", "secret-b", auth.RoleClient)
	tokenA := h.mustLogin("client-a", "secret-a")
	jsonHeaders := map[string]string{"Content-Type": "application/json"}

	// Operator files one request per client plus one unassigned.
	create := func(clientID string) request.MaterialRequest {
		h.t.Helper()
		rr := h.do(http.MethodPost, "/api/v1/requests",
			`{"product_id":"`+p.ID+`","usage":"listing","client_id":"`+clientID+`"}`, jsonHeaders)
		if rr.Code != http.StatusCreated {
			h.t.Fatalf("create request: expected 201 got %d body=%s", rr.Code, rr.Body.String())
		}
		var created request.MaterialRequest
		if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
			h.t.Fatal(err)
		}
		return created
	}
	create(clientA.ID)
	other := create("client-b-id")
	create("")

	// The client sees the full, unfiltered list.
	rr := h.doAs(tokenA, http.MethodGet, "/api/v1/requests", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var list struct {
		Items      []request.MaterialRequest `json:"items"`
		Pagination struct {
			TotalItems int `json:"total_items"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Pagination.TotalItems != 3 || len(list.Items) != 3 {
		t.Fatalf("expected all 3 requests got %#v", list)
	}

	// A client-supplied client_id is honored as-is.
	rr = h.doAs(tokenA, http.MethodPost, "/api/v1/requests",
		`{"product_id":"`+p.ID+`","usage":"ad","client_id":"spoofed"}`, jsonHeaders)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d body=%s", rr.Code, rr.Body.String())
	}
	var spoofed request.MaterialRequest
	if err := json.Unmarshal(rr.Body.Bytes(), &spoofed); err != nil {
		t.Fatal(err)
	}
	if spoofed.ClientID != "spoofed" {
		t.Fatalf("expected client_id from body got %q", spoofed.ClientID)
	}

	// Reading somebody else's request is allowed.
	rr = h.doAs(tokenA, http.MethodGet, "/api/v1/requests/"+other.ID, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestClientCanActOnOthersRequest(t *testing.T) {
	// 单端模式：client 可生成、审批、打回、查看任何需求；actor 仍强制为
	// 登录用户名。
	h := newHarness(t)
	p := h.mustProductWithDetail("护眼台灯")
	clientA := h.mustUser("client-a", "secret-a", auth.RoleClient)
	tokenA := h.mustLogin("client-a", "secret-a")
	jsonHeaders := map[string]string{"Content-Type": "application/json"}

	// An operator-owned request (no client_id) is fully reachable.
	req := h.mustRequest(p.ID, "短视频带货")
	variants := h.mustGenerate(req.ID)

	rr := h.doAs(tokenA, http.MethodGet, "/api/v1/requests/"+req.ID+"/variants", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("variants: expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.doAs(tokenA, http.MethodPost, "/api/v1/requests/"+req.ID+"/approve",
		`{"variant_id":"`+variants[0].ID+`","actor":"spoofed"}`, jsonHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("approve: expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var approveResp struct {
		Event review.ReviewEvent `json:"event"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &approveResp); err != nil {
		t.Fatal(err)
	}
	if approveResp.Event.Actor != clientA.Username {
		t.Fatalf("expected actor forced to %q got %q", clientA.Username, approveResp.Event.Actor)
	}

	rr = h.doAs(tokenA, http.MethodGet, "/api/v1/requests/"+req.ID+"/review", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("review: expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}

	// Reject works too, on a fresh operator-owned request.
	other := h.mustRequest(p.ID, "短视频带货")
	h.mustGenerate(other.ID)
	rr = h.doAs(tokenA, http.MethodPost, "/api/v1/requests/"+other.ID+"/reject",
		`{"reason":"不行"}`, jsonHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("reject: expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestOperatorActorDefaultsToUsername(t *testing.T) {
	h := newHarness(t)
	p := h.mustProductWithDetail("护眼台灯")
	req := h.mustRequest(p.ID, "短视频带货")
	variants := h.mustGenerate(req.ID)

	rr := h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/approve",
		`{"variant_id":"`+variants[0].ID+`"}`,
		map[string]string{"Content-Type": "application/json"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Event review.ReviewEvent `json:"event"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Event.Actor != "admin" {
		t.Fatalf("expected default actor admin got %q", resp.Event.Actor)
	}
}
