package apiv1

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"tiktok-trend-shop/internal/auth"
	"tiktok-trend-shop/internal/category"
	"tiktok-trend-shop/internal/copygen"
	"tiktok-trend-shop/internal/dossier"
	"tiktok-trend-shop/internal/importer"
	"tiktok-trend-shop/internal/mediagen"
	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/request"
	"tiktok-trend-shop/internal/review"
	"tiktok-trend-shop/internal/score"
)

// Server bundles dependencies for the /api/v1 routes.
type Server struct {
	db       *sql.DB
	product  *product.Repository
	category *category.Repository
	importer *importer.Importer
	score    *score.Repository
	request  *request.Repository
	copygen  *copygen.Service
	review   *review.Service
	auth     *auth.Service
	dossier  *dossier.Repository
	mediagen *mediagen.Service
}

// NewServer constructs a Server with the supplied dependencies.
func NewServer(db *sql.DB, pr *product.Repository, cat *category.Repository, imp *importer.Importer, sc *score.Repository, rq *request.Repository, cg *copygen.Service, rv *review.Service, au *auth.Service, ds *dossier.Repository, mg *mediagen.Service) *Server {
	return &Server{db: db, product: pr, category: cat, importer: imp, score: sc, request: rq, copygen: cg, review: rv, auth: au, dossier: ds, mediagen: mg}
}

// Handler returns the HTTP handler for the /api/v1 routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.HandleFunc("GET /api/v1/auth/me", s.me)
	mux.HandleFunc("POST /api/v1/users", s.operatorOnly(s.createUser))
	mux.HandleFunc("GET /api/v1/users", s.operatorOnly(s.listUsers))
	mux.HandleFunc("GET /api/v1/trends", s.listTrends)
	mux.HandleFunc("GET /api/v1/products/{id}", s.getProduct)
	mux.HandleFunc("GET /api/v1/products/{id}/metrics", s.getProductMetrics)
	mux.HandleFunc("GET /api/v1/categories", s.listCategories)
	mux.HandleFunc("GET /api/v1/marketplaces", s.listMarketplaces)
	mux.HandleFunc("GET /api/v1/category-mappings", s.operatorOnly(s.listMappings))
	mux.HandleFunc("GET /api/v1/products/{id}/category", s.operatorOnly(s.getProductCategory))
	mux.HandleFunc("PATCH /api/v1/products/{id}/category", s.operatorOnly(s.assignProductCategory))
	mux.HandleFunc("GET /api/v1/imports", s.operatorOnly(s.listImports))
	mux.HandleFunc("POST /api/v1/imports/csv", s.operatorOnly(s.createImport))
	mux.HandleFunc("GET /api/v1/imports/{id}", s.operatorOnly(s.getImport))
	mux.HandleFunc("GET /api/v1/products/{id}/score", s.getProductScore)
	mux.HandleFunc("POST /api/v1/requests", s.createRequest)
	mux.HandleFunc("GET /api/v1/requests", s.listRequests)
	mux.HandleFunc("GET /api/v1/requests/{id}", s.getRequest)
	mux.HandleFunc("POST /api/v1/products/{id}/prefill", s.prefillRequestBrief)
	mux.HandleFunc("POST /api/v1/requests/{id}/generate", s.operatorOnly(s.generateRequestCopy))
	mux.HandleFunc("GET /api/v1/requests/{id}/variants", s.listRequestVariants)
	mux.HandleFunc("POST /api/v1/requests/{id}/approve", s.approveRequest)
	mux.HandleFunc("POST /api/v1/requests/{id}/reject", s.rejectRequest)
	mux.HandleFunc("POST /api/v1/requests/{id}/deliver", s.operatorOnly(s.deliverRequest))
	mux.HandleFunc("GET /api/v1/requests/{id}/review", s.getRequestReview)
	mux.HandleFunc("GET /api/v1/deliveries", s.operatorOnly(s.listDeliveries))
	mux.HandleFunc("GET /api/v1/deliveries/{id}", s.operatorOnly(s.getDelivery))
	mux.HandleFunc("POST /api/v1/products/{id}/assets", s.createDossierAsset)
	mux.HandleFunc("GET /api/v1/products/{id}/assets", s.listDossierAssets)
	mux.HandleFunc("DELETE /api/v1/products/{id}/assets/{asset_id}", s.deleteDossierAsset)
	mux.HandleFunc("GET /api/v1/mediagen/health", s.mediagenHealth)
	mux.HandleFunc("POST /api/v1/products/{id}/images/generate", s.operatorOnly(s.generateProductImage))
	mux.HandleFunc("GET /api/v1/products/{id}/images", s.listProductImages)
	return jsonMiddleware(s.requireAuth(mux))
}

func (s *Server) listTrends(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := ParsePage(q.Get("page"))
	pageSize := ParsePageSize(q.Get("page_size"))
	sortKey := ParseSort(q.Get("sort"), map[string]string{
		"score":         "score",
		"price":         "price",
		"updated":       "updated",
		"created":       "created",
		"asin":          "asin",
	}, "score")
	direction := ParseDirection(q.Get("direction"))
	search := strings.TrimSpace(q.Get("q"))
	canonical := strings.TrimSpace(q.Get("category"))
	marketplace := strings.TrimSpace(q.Get("marketplace"))
	window := ParseTimeWindow(q.Get("window"), "30d")
	staleHours := 0
	if q.Get("stale") != "" {
		if v, err := atoiOrZero(q.Get("stale")); err == nil {
			staleHours = v
		}
	}

	results, total, err := s.queryTrends(r.Context(), trendsQuery{
		Page:         page,
		PageSize:     pageSize,
		Sort:         sortKey,
		Direction:    direction,
		Search:       search,
		Canonical:    canonical,
		Marketplace:  marketplace,
		Window:       window,
		StaleHours:   staleHours,
	})
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, ListResponse{
		Items:     results,
		Pagination: PaginationFor(page, pageSize, total),
		Effective: map[string]any{
			"sort":         sortKey,
			"direction":    direction,
			"page":         page,
			"page_size":    pageSize,
			"marketplace":  marketplace,
			"category":     canonical,
			"window":       window,
			"q":            search,
		},
	})
}

func (s *Server) getProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	p, err := s.product.GetProduct(r.Context(), id)
	if err == sql.ErrNoRows {
		WriteNotFound(w, "product")
		return
	}
	if err != nil {
		WriteInternal(w, err)
		return
	}
	detail, detailOK, _ := s.product.LatestDetail(r.Context(), id)
	snap, snapOK, _ := s.product.LatestSnapshot(r.Context(), id)
	scoreSnap, scoreOK, _ := s.score.LatestForProduct(r.Context(), id)
	assignment, assignmentOK, _ := s.category.GetAssignment(r.Context(), id)
	var detailVal, snapVal, scoreVal, assignmentVal any
	if detailOK {
		detailVal = detail
	}
	if snapOK {
		snapVal = snap
	}
	if scoreOK {
		scoreVal = scoreSnap
	}
	if assignmentOK {
		assignmentVal = assignment
	}
	resp := map[string]any{
		"product":         p,
		"detail":          detailVal,
		"latest_snapshot": snapVal,
		"score":           scoreVal,
		"assignment":      assignmentVal,
	}
	WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) getProductMetrics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	window := ParseTimeWindow(r.URL.Query().Get("window"), "30d")
	limit := ParsePageSize(r.URL.Query().Get("limit"))
	if limit > 200 {
		limit = 200
	}
	history, err := s.product.SnapshotHistory(r.Context(), id, window, limit)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"product_id": id,
		"window":     window,
		"items":      history,
	})
}

func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := s.category.ListCanonical(r.Context())
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, ListResponse{Items: cats, Pagination: PaginationFor(1, MaxPageSize, len(cats))})
}

func (s *Server) listMarketplaces(w http.ResponseWriter, r *http.Request) {
	marketplaces, err := s.product.ListMarketplaces(r.Context())
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, ListResponse{Items: marketplaces, Pagination: PaginationFor(1, MaxPageSize, len(marketplaces))})
}

func (s *Server) listMappings(w http.ResponseWriter, r *http.Request) {
	mappings, err := s.category.ListMappings(r.Context())
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, ListResponse{Items: mappings, Pagination: PaginationFor(1, MaxPageSize, len(mappings))})
}

func (s *Server) getProductCategory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a, ok, err := s.category.GetAssignment(r.Context(), id)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	if !ok {
		WriteJSON(w, http.StatusOK, map[string]any{"product_id": id, "assignment": nil})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"product_id": id, "assignment": a})
}

func (s *Server) assignProductCategory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	var body struct {
		CanonicalCategoryID string `json:"canonical_category_id"`
		Reviewer            string `json:"reviewer"`
	}
	if err := decodeJSON(r, &body); err != nil {
		WriteFieldError(w, "body", "invalid JSON body")
		return
	}
	if body.CanonicalCategoryID == "" {
		WriteFieldError(w, "canonical_category_id", "canonical_category_id is required")
		return
	}
	if body.Reviewer == "" {
		body.Reviewer = "workbench"
	}
	a, err := s.category.AssignManually(r.Context(), id, body.CanonicalCategoryID, body.Reviewer)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"product_id": id, "assignment": a})
}

func (s *Server) listImports(w http.ResponseWriter, r *http.Request) {
	limit := ParsePageSize(r.URL.Query().Get("limit"))
	if limit > 100 {
		limit = 100
	}
	jobs, err := s.importer.ListJobs(r.Context(), limit)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, ListResponse{Items: jobs, Pagination: PaginationFor(1, limit, len(jobs))})
}

func (s *Server) getImport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, rows, err := s.importer.GetJob(r.Context(), id)
	if err == sql.ErrNoRows {
		WriteNotFound(w, "import_job")
		return
	}
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"job":     job,
		"results": rows,
	})
}

func (s *Server) getProductScore(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	snap, ok, err := s.score.LatestForProduct(r.Context(), id)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	if !ok {
		WriteNotFound(w, "score")
		return
	}
	WriteJSON(w, http.StatusOK, snap)
}

func (s *Server) createRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProductID string `json:"product_id"`
		ClientID  string `json:"client_id"`
		Usage     string `json:"usage"`
		Style     string `json:"style"`
		Focus     string `json:"focus"`
		Notes     string `json:"notes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		WriteFieldError(w, "body", "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.ProductID) == "" {
		WriteFieldError(w, "product_id", "product_id is required")
		return
	}
	if _, err := s.product.GetProduct(r.Context(), body.ProductID); err == sql.ErrNoRows {
		WriteNotFound(w, "product")
		return
	} else if err != nil {
		WriteInternal(w, err)
		return
	}
	// 单端模式：不再强制 client_id=自身，body 传什么用什么（可空）。
	// 恢复 RBAC 时还原为：登录用户为 client 角色时 clientID = user.ID。
	clientID := strings.TrimSpace(body.ClientID)
	req, err := s.request.Create(r.Context(), request.RequestInput{
		ProductID: body.ProductID,
		ClientID:  clientID,
		Usage:     strings.TrimSpace(body.Usage),
		Style:     strings.TrimSpace(body.Style),
		Focus:     strings.TrimSpace(body.Focus),
		Notes:     strings.TrimSpace(body.Notes),
	})
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, req)
}

func (s *Server) listRequests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := ParsePage(q.Get("page"))
	pageSize := ParsePageSize(q.Get("page_size"))
	status := strings.TrimSpace(q.Get("status"))
	productID := strings.TrimSpace(q.Get("product_id"))
	filter := request.ListFilter{
		Status:    status,
		ProductID: productID,
		Page:      page,
		PageSize:  pageSize,
	}
	// 单端模式：不再按 client_id 过滤，所有登录用户见全量。
	// 恢复 RBAC 时还原为：client 角色用户设置 filter.ClientID = user.ID。
	items, total, err := s.request.List(r.Context(), filter)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, ListResponse{
		Items:      items,
		Pagination: PaginationFor(page, pageSize, total),
		Effective: map[string]any{
			"page":       page,
			"page_size":  pageSize,
			"status":     status,
			"product_id": productID,
		},
	})
}

func (s *Server) getRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	req, err := s.request.Get(r.Context(), id)
	if err == sql.ErrNoRows {
		WriteNotFound(w, "material_request")
		return
	}
	if err != nil {
		WriteInternal(w, err)
		return
	}
	if !clientOwnsRequest(w, r, req.ClientID) {
		return
	}
	WriteJSON(w, http.StatusOK, req)
}

func (s *Server) prefillRequestBrief(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	suggestion, err := s.copygen.PrefillForProduct(r.Context(), id)
	if err == sql.ErrNoRows {
		WriteNotFound(w, "product")
		return
	}
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"product_id": id, "suggestion": suggestion})
}

func (s *Server) generateRequestCopy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	variants, err := s.copygen.GenerateForRequest(r.Context(), id)
	if err == sql.ErrNoRows {
		WriteNotFound(w, "material_request")
		return
	}
	var statusErr *copygen.StatusError
	if errors.As(err, &statusErr) {
		WriteError(w, http.StatusConflict, "invalid_status", statusErr.Error())
		return
	}
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"request_id": id, "items": variants})
}

func (s *Server) listRequestVariants(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	req, err := s.request.Get(r.Context(), id)
	if err == sql.ErrNoRows {
		WriteNotFound(w, "material_request")
		return
	} else if err != nil {
		WriteInternal(w, err)
		return
	}
	if !clientOwnsRequest(w, r, req.ClientID) {
		return
	}
	variants, err := s.copygen.ListVariants(r.Context(), id)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"request_id": id, "items": variants})
}

func (s *Server) approveRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	if !s.requestAccessible(w, r, id) {
		return
	}
	var body struct {
		VariantID string `json:"variant_id"`
		Actor     string `json:"actor"`
		Note      string `json:"note"`
	}
	if err := decodeJSON(r, &body); err != nil {
		WriteFieldError(w, "body", "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.VariantID) == "" {
		WriteFieldError(w, "variant_id", "variant_id is required")
		return
	}
	event, err := s.review.Approve(r.Context(), id,
		strings.TrimSpace(body.VariantID), resolveActor(r, strings.TrimSpace(body.Actor)), strings.TrimSpace(body.Note))
	if err != nil {
		writeReviewError(w, err)
		return
	}
	req, err := s.request.Get(r.Context(), id)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"request": req, "event": event})
}

func (s *Server) rejectRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	if !s.requestAccessible(w, r, id) {
		return
	}
	var body struct {
		Reason string `json:"reason"`
		Actor  string `json:"actor"`
	}
	if err := decodeJSON(r, &body); err != nil {
		WriteFieldError(w, "body", "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.Reason) == "" {
		WriteFieldError(w, "reason", "reason is required")
		return
	}
	event, err := s.review.Reject(r.Context(), id,
		resolveActor(r, strings.TrimSpace(body.Actor)), strings.TrimSpace(body.Reason))
	if err != nil {
		writeReviewError(w, err)
		return
	}
	req, err := s.request.Get(r.Context(), id)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"request": req, "event": event})
}

func (s *Server) deliverRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	var body struct {
		VariantID string `json:"variant_id"`
		Actor     string `json:"actor"`
	}
	if err := decodeJSON(r, &body); err != nil {
		WriteFieldError(w, "body", "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.VariantID) == "" {
		WriteFieldError(w, "variant_id", "variant_id is required")
		return
	}
	delivery, err := s.review.Deliver(r.Context(), id,
		strings.TrimSpace(body.VariantID), resolveActor(r, strings.TrimSpace(body.Actor)))
	if err != nil {
		writeReviewError(w, err)
		return
	}
	req, err := s.request.Get(r.Context(), id)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"request": req, "delivery": delivery})
}

func (s *Server) getRequestReview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	req, err := s.request.Get(r.Context(), id)
	if err == sql.ErrNoRows {
		WriteNotFound(w, "material_request")
		return
	}
	if err != nil {
		WriteInternal(w, err)
		return
	}
	if !clientOwnsRequest(w, r, req.ClientID) {
		return
	}
	events, err := s.review.ListEvents(r.Context(), id)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	var delivery *review.Delivery
	if d, err := s.review.GetDelivery(r.Context(), id); err == nil {
		delivery = &d
	} else if err != sql.ErrNoRows {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"request":  req,
		"events":   events,
		"delivery": delivery,
	})
}

func (s *Server) listDeliveries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := ParsePage(q.Get("page"))
	pageSize := ParsePageSize(q.Get("page_size"))
	items, total, err := s.review.ListDeliveries(r.Context(), page, pageSize)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, ListResponse{
		Items:      items,
		Pagination: PaginationFor(page, pageSize, total),
		Effective: map[string]any{
			"page":      page,
			"page_size": pageSize,
		},
	})
}

func (s *Server) getDelivery(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	delivery, err := s.review.GetDeliveryByID(r.Context(), id)
	if err == sql.ErrNoRows {
		WriteNotFound(w, "delivery")
		return
	}
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, delivery)
}

// writeReviewError maps review service errors onto the standard envelope:
// missing requests 404, illegal status 409 invalid_status, repeated delivery
// 409 already_delivered, and variant mismatches a 400 field error.
func writeReviewError(w http.ResponseWriter, err error) {
	if err == sql.ErrNoRows {
		WriteNotFound(w, "material_request")
		return
	}
	var statusErr *review.StatusError
	if errors.As(err, &statusErr) {
		WriteError(w, http.StatusConflict, "invalid_status", statusErr.Error())
		return
	}
	if errors.Is(err, review.ErrAlreadyDelivered) {
		WriteError(w, http.StatusConflict, "already_delivered", err.Error())
		return
	}
	var variantErr *review.VariantError
	if errors.As(err, &variantErr) {
		WriteFieldError(w, "variant_id", variantErr.Error())
		return
	}
	WriteInternal(w, err)
}

func (s *Server) createImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(importer.MaxBytes); err != nil {
		WriteFieldError(w, "file", "invalid multipart payload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		WriteFieldError(w, "file", "file is required")
		return
	}
	defer file.Close()
	if header.Size > importer.MaxBytes {
		WriteFieldError(w, "file", "file exceeds size limit")
		return
	}
	defaultRegion := r.FormValue("region")
	if defaultRegion == "" {
		defaultRegion = "CN"
	}
	result, err := s.importer.Import(r.Context(), file, importer.Options{
		Source:         "manual-csv",
		Filename:       header.Filename,
		IdempotencyKey: r.FormValue("idempotency_key"),
		MaxBytes:       importer.MaxBytes,
		MaxRows:        importer.MaxRows,
	}, defaultRegion)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "import_error", err.Error())
		return
	}
	WriteJSON(w, http.StatusCreated, result)
}

type trendsQuery struct {
	Page        int
	PageSize    int
	Sort        string
	Direction   string
	Search      string
	Canonical   string
	Marketplace string
	Window      string
	StaleHours  int
}

// queryTrends returns paginated products using the standard filters and
// attaches the latest hotspot score snapshot for each row.
func (s *Server) queryTrends(ctx context.Context, q trendsQuery) ([]product.ProductResult, int, error) {
	offset := (q.Page - 1) * q.PageSize
	where := []string{"1=1"}
	args := []any{}
	if q.Marketplace != "" {
		where = append(where, "(p.marketplace = ? OR p.region = ?)")
		args = append(args, q.Marketplace, q.Marketplace)
	}
	if q.Canonical != "" {
		where = append(where, `EXISTS (
			SELECT 1 FROM product_category_assignments a
			WHERE a.product_id = p.id AND a.canonical_category_id = ?
		)`)
		args = append(args, q.Canonical)
	}
	if q.Search != "" {
		where = append(where, "(LOWER(p.title) LIKE ? OR LOWER(COALESCE(p.asin, '')) LIKE ?)")
		like := "%" + strings.ToLower(q.Search) + "%"
		args = append(args, like, like)
	}
	whereSQL := strings.Join(where, " AND ")

	// Order
	order := "p.created_at DESC"
	switch q.Sort {
	case "score":
		order = "COALESCE(hs.total_score, 0) " + q.Direction + ", p.created_at DESC"
	case "price":
		order = "COALESCE((SELECT price FROM product_detail_snapshots d WHERE d.product_id = p.id ORDER BY captured_at DESC LIMIT 1), 0) " + q.Direction + ", p.created_at DESC"
	case "updated":
		order = "p.updated_at " + q.Direction
	case "asin":
		order = "p.asin " + q.Direction
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM products p WHERE " + whereSQL
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Page query joining the latest score snapshot (if any)
	dataQuery := `
		SELECT p.id, p.canonical_url, p.title, p.region, p.marketplace, p.asin,
			p.source_category, p.source_category_id, p.category, p.workflow_state, p.metadata_json,
			p.last_captured_at, hs.id, hs.total_score, hs.confidence, hs.model_version,
			hs.time_window, hs.components_json, hs.missing_components_json, hs.created_at
		FROM products p
		LEFT JOIN hotspot_scores hs ON hs.id = (
			SELECT id FROM hotspot_scores
			WHERE product_id = p.id
			ORDER BY created_at DESC LIMIT 1
		)
		WHERE ` + whereSQL + `
		ORDER BY ` + order + `
		LIMIT ? OFFSET ?
	`
	pageArgs := append([]any{}, args...)
	pageArgs = append(pageArgs, q.PageSize, offset)
	rows, err := s.db.QueryContext(ctx, dataQuery, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var results []product.ProductResult
	for rows.Next() {
		var (
			result       product.ProductResult
			metadataJSON sql.NullString
			canonicalURL sql.NullString
			marketplace  sql.NullString
			asin         sql.NullString
			sourceCat    sql.NullString
			sourceCatID  sql.NullString
			categoryCol  sql.NullString
			lastCaptured sql.NullString
			hsID         sql.NullString
			hsTotal      sql.NullFloat64
			hsConf       sql.NullString
			hsVersion    sql.NullString
			hsWindow     sql.NullString
			hsComponents sql.NullString
			hsMissing    sql.NullString
			hsCreated    sql.NullString
		)
		if err := rows.Scan(
			&result.ID, &canonicalURL, &result.Title, &result.Region, &marketplace, &asin,
			&sourceCat, &sourceCatID, &categoryCol, &result.WorkflowState, &metadataJSON,
			&lastCaptured, &hsID, &hsTotal, &hsConf, &hsVersion,
			&hsWindow, &hsComponents, &hsMissing, &hsCreated,
		); err != nil {
			return nil, 0, err
		}
		result.CanonicalURL = nullStringPtr(canonicalURL)
		result.Marketplace = nullStringPtr(marketplace)
		result.ASIN = nullStringPtr(asin)
		result.SourceCategory = nullStringPtr(sourceCat)
		result.SourceCategoryID = nullStringPtr(sourceCatID)
		result.Category = nullStringPtr(categoryCol)
		result.Metadata = decodeMap(metadataJSON.String)
		if lastCaptured.Valid {
			ts := lastCaptured.String
			result.Freshness.LastCapturedAt = &ts
			result.Freshness.ThresholdHours = product.DefaultStalenessHours
			result.Freshness.Stale = product.IsStaleTime(ts, product.DefaultStalenessHours)
		}
		result.Metrics = s.product.LatestMetrics(ctx, result.ID)
		result.MetricKind, result.Estimated = s.product.LatestMetricKind(ctx, result.ID)
		result.Provenance = s.product.LatestProvenance(ctx, result.ID)
		s.product.AttachLatestDetail(ctx, &result)
		if hsID.Valid && hsTotal.Valid {
			score := product.ScoreSummary{
				ID:                hsID.String,
				TotalScore:        hsTotal.Float64,
				Confidence:        hsConf.String,
				ModelVersion:      hsVersion.String,
				TimeWindow:        hsWindow.String,
				Components:        decodeFloatMap(hsComponents.String),
				MissingComponents: decodeItems(hsMissing.String),
				CreatedAt:         hsCreated.String,
			}
			result.Score = &score
		}
		results = append(results, result)
	}
	return results, total, rows.Err()
}

func decodeMap(s string) map[string]any {
	if s == "" {
		return map[string]any{}
	}
	out := map[string]any{}
	_ = jsonUnmarshal(s, &out)
	return out
}

func decodeFloatMap(s string) map[string]float64 {
	if s == "" {
		return map[string]float64{}
	}
	out := map[string]float64{}
	_ = jsonUnmarshal(s, &out)
	return out
}

func decodeItems(s string) []string {
	if s == "" {
		return nil
	}
	var out struct {
		Items []string `json:"items"`
	}
	_ = jsonUnmarshal(s, &out)
	return out.Items
}

func nullStringPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

func atoiOrZero(value string) (int, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return 0, nil
	}
	var n int
	for _, c := range v {
		if c < '0' || c > '9' {
			return 0, errInvalid
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

var errInvalid = errInvalidParam{}

type errInvalidParam struct{}

func (errInvalidParam) Error() string { return "invalid numeric parameter" }