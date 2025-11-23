package repository

import (
	"cloud-function-event/internal/domain"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	CollectionEvents   = "events"
	CollectionTracking = "tracking"
)

// Repositories container to hold all specific repos [NEW]
type Repositories struct {
	Events   EventRepository
	Tracking TrackingRepository
}

// EventRepository defines the interface for Event DB interactions
type EventRepository interface {
	Save(ctx context.Context, event *domain.Event) error
	Update(ctx context.Context, id string, updates map[string]interface{}) error
	GetByID(ctx context.Context, id string) (*domain.Event, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, search domain.SearchRequest) ([]domain.Event, string, error)
}

// TrackingRepository defines the interface for Tracking DB interactions [NEW]
type TrackingRepository interface {
	SaveTracking(ctx context.Context, tracking *domain.TrackingEvent) error
	ListTracking(ctx context.Context) ([]domain.TrackingEvent, error)
}

type firestoreRepo struct {
	client *firestore.Client
}

func NewFirestoreRepository(client *firestore.Client) *Repositories {
	repo := &firestoreRepo{client: client}
	return &Repositories{
		Events:   repo,
		Tracking: repo,
	}
}

func (r *firestoreRepo) Save(ctx context.Context, event *domain.Event) error {
	_, err := r.client.Collection(CollectionEvents).Doc(event.ID).Set(ctx, event)
	return err
}

func (r *firestoreRepo) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	_, err := r.client.Collection(CollectionEvents).Doc(id).Set(ctx, updates, firestore.MergeAll)
	return err
}

func (r *firestoreRepo) GetByID(ctx context.Context, id string) (*domain.Event, error) {
	doc, err := r.client.Collection(CollectionEvents).Doc(id).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return nil, fmt.Errorf("event not found")
	}
	if err != nil {
		return nil, err
	}

	var event domain.Event
	if err := doc.DataTo(&event); err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *firestoreRepo) Delete(ctx context.Context, id string) error {
	_, err := r.client.Collection(CollectionEvents).Doc(id).Delete(ctx)
	return err
}

// cursorData struct to encode in the PageToken
type cursorData struct {
	SortValue interface{} `json:"v"`
	ID        string      `json:"id"`
}

func (r *firestoreRepo) List(ctx context.Context, search domain.SearchRequest) ([]domain.Event, string, error) {
	query := r.client.Collection(CollectionEvents).Query

	// 1. Apply Filters
	f := search.Filters
	if f.City != "" {
		query = query.Where("city", "==", f.City)
	}
	if f.Type != "" {
		query = query.Where("type", "==", f.Type)
	}
	if f.StartDate != nil {
		query = query.Where("start_time", ">=", *f.StartDate)
	}
	if f.EndDate != nil {
		query = query.Where("end_time", "<=", *f.EndDate)
	}
	if f.MinPrice != nil {
		query = query.Where("price", ">=", *f.MinPrice)
	}
	if f.MaxPrice != nil {
		query = query.Where("price", "<=", *f.MaxPrice)
	}

	// 2. Apply Sorting
	// Default sort: start_time ASC (soonest events first)
	sortKey := "start_time"
	dir := firestore.Asc

	if search.Sorting.SortKey != "" {
		sortKey = search.Sorting.SortKey
	}

	if search.Sorting.SortDirection == "desc" {
		dir = firestore.Desc
	}

	// Order by SortKey then by ID for stable cursor pagination
	query = query.OrderBy(sortKey, dir).OrderBy("id", dir)

	// 3. Apply Pagination
	pageSize := 10
	if search.Sorting.PageSize > 0 {
		pageSize = search.Sorting.PageSize
	}

	// Limit to pageSize
	query = query.Limit(pageSize)

	// Cursor: StartAfter
	if search.Sorting.PageToken != "" {
		vals, err := decodeCursor(search.Sorting.PageToken)
		if err == nil {
			query = query.StartAfter(vals...)
		}
	}

	// Execute
	iter := query.Documents(ctx)
	defer iter.Stop()

	events := []domain.Event{}
	var lastDoc *firestore.DocumentSnapshot

	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", err
		}
		var e domain.Event
		if err := doc.DataTo(&e); err != nil {
			return nil, "", fmt.Errorf("failed to map document: %v", err)
		}
		events = append(events, e)
		lastDoc = doc
	}

	// Generate Next Page Token
	nextPageToken := ""
	if len(events) == pageSize && lastDoc != nil {
		val, err := lastDoc.DataAt(sortKey)
		if err == nil {
			nextPageToken = encodeCursor(val, lastDoc.Ref.ID)
		}
	}

	return events, nextPageToken, nil
}

// --- Tracking Implementation [NEW] ---

func (r *firestoreRepo) SaveTracking(ctx context.Context, tracking *domain.TrackingEvent) error {
	// Use the ID provided or generate a new doc
	if tracking.ID != "" {
		_, err := r.client.Collection(CollectionTracking).Doc(tracking.ID).Set(ctx, tracking)
		return err
	}
	_, _, err := r.client.Collection(CollectionTracking).Add(ctx, tracking)
	return err
}

func (r *firestoreRepo) ListTracking(ctx context.Context) ([]domain.TrackingEvent, error) {
	// Simple Get All, ordered by CreatedAt desc
	iter := r.client.Collection(CollectionTracking).OrderBy("created_at", firestore.Desc).Documents(ctx)
	defer iter.Stop()

	var tracks []domain.TrackingEvent
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		var t domain.TrackingEvent
		if err := doc.DataTo(&t); err != nil {
			continue
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

func encodeCursor(sortVal interface{}, id string) string {
	data := cursorData{SortValue: sortVal, ID: id}
	b, _ := json.Marshal(data)
	return base64.StdEncoding.EncodeToString(b)
}

func decodeCursor(token string) ([]interface{}, error) {
	b, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	var data cursorData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}

	// Handle time unmarshalling (JSON numbers/strings -> time.Time)
	// If we sort by start_time, SortValue will be a string in JSON
	if s, ok := data.SortValue.(string); ok {
		// Try parsing as time
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return []interface{}{t, data.ID}, nil
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return []interface{}{t, data.ID}, nil
		}
	}

	return []interface{}{data.SortValue, data.ID}, nil
}
