package review

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"tiktok-trend-shop/internal/id"
)

// ErrAlreadyDelivered reports that a delivery already exists for the request.
// The deliveries.request_id UNIQUE constraint is the source of truth.
var ErrAlreadyDelivered = errors.New("request already delivered")

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *sql.DB { return r.db }

// EventInput carries the fields of a new review event.
type EventInput struct {
	RequestID string
	Action    string
	Actor     string
	Note      string
	VariantID *string
}

// AddEvent persists one approval-flow event and returns the stored row.
func (r *Repository) AddEvent(ctx context.Context, input EventInput) (ReviewEvent, error) {
	eventID := id.New("rev")
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO review_events(id, request_id, action, actor, note, variant_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, eventID, input.RequestID, input.Action, input.Actor, input.Note,
		nullString(input.VariantID), utcNow())
	if err != nil {
		return ReviewEvent{}, err
	}
	return r.getEvent(ctx, eventID)
}

func (r *Repository) getEvent(ctx context.Context, eventID string) (ReviewEvent, error) {
	row := r.db.QueryRowContext(ctx, eventSelectColumns+` WHERE id = ?`, eventID)
	return scanEvent(row)
}

// ListEvents returns the events of a request in chronological order. rowid
// breaks ties between events written within the same second.
func (r *Repository) ListEvents(ctx context.Context, requestID string) ([]ReviewEvent, error) {
	rows, err := r.db.QueryContext(ctx, eventSelectColumns+`
		WHERE request_id = ?
		ORDER BY created_at ASC, rowid ASC
	`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []ReviewEvent{}
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, event)
	}
	return results, rows.Err()
}

// DeliveryInput carries the fields of a new delivery.
type DeliveryInput struct {
	RequestID   string
	VariantID   string
	Actor       string
	PackageJSON string
}

// CreateDelivery persists the delivery package of a request. A request may be
// delivered only once: a UNIQUE conflict on request_id returns
// ErrAlreadyDelivered.
func (r *Repository) CreateDelivery(ctx context.Context, input DeliveryInput) (Delivery, error) {
	deliveryID := id.New("dlv")
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO deliveries(id, request_id, variant_id, actor, package_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, deliveryID, input.RequestID, input.VariantID, input.Actor, input.PackageJSON, utcNow())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: deliveries.request_id") {
			return Delivery{}, ErrAlreadyDelivered
		}
		return Delivery{}, err
	}
	return r.GetDelivery(ctx, input.RequestID)
}

// GetDelivery returns the delivery of a request, or sql.ErrNoRows when the
// request has not been delivered yet.
func (r *Repository) GetDelivery(ctx context.Context, requestID string) (Delivery, error) {
	row := r.db.QueryRowContext(ctx, deliverySelectColumns+` WHERE request_id = ?`, requestID)
	return scanDelivery(row)
}

// GetDeliveryByID returns one delivery by its own id, or sql.ErrNoRows.
func (r *Repository) GetDeliveryByID(ctx context.Context, deliveryID string) (Delivery, error) {
	row := r.db.QueryRowContext(ctx, deliverySelectColumns+` WHERE id = ?`, deliveryID)
	return scanDelivery(row)
}

// ListDeliveries returns the filtered page of deliveries plus the total count
// across all pages.
func (r *Repository) ListDeliveries(ctx context.Context, page, pageSize int) ([]Delivery, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM deliveries`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, deliverySelectColumns+`
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	results := []Delivery{}
	for rows.Next() {
		delivery, err := scanDelivery(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, delivery)
	}
	return results, total, rows.Err()
}

const eventSelectColumns = `
	SELECT id, request_id, action, actor, note, variant_id, created_at
	FROM review_events
`

const deliverySelectColumns = `
	SELECT id, request_id, variant_id, actor, package_json, created_at
	FROM deliveries
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(row rowScanner) (ReviewEvent, error) {
	var (
		event     ReviewEvent
		variantID sql.NullString
	)
	if err := row.Scan(
		&event.ID,
		&event.RequestID,
		&event.Action,
		&event.Actor,
		&event.Note,
		&variantID,
		&event.CreatedAt,
	); err != nil {
		return ReviewEvent{}, err
	}
	if variantID.Valid {
		event.VariantID = &variantID.String
	}
	return event, nil
}

func scanDelivery(row rowScanner) (Delivery, error) {
	var (
		delivery    Delivery
		packageJSON string
	)
	if err := row.Scan(
		&delivery.ID,
		&delivery.RequestID,
		&delivery.VariantID,
		&delivery.Actor,
		&packageJSON,
		&delivery.CreatedAt,
	); err != nil {
		return Delivery{}, err
	}
	delivery.Package = json.RawMessage(packageJSON)
	return delivery, nil
}

func nullString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func utcNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}
