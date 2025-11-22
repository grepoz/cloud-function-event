package repository

import (
	"cloud-function-event/internal/domain"
	"context"
	"fmt"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const CollectionName = "events"

// EventRepository defines the interface for DB interactions
type EventRepository interface {
	Save(ctx context.Context, event *domain.Event) error
	// Update now accepts a map for partial updates
	Update(ctx context.Context, id string, updates map[string]interface{}) error
	GetByID(ctx context.Context, id string) (*domain.Event, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, search domain.SearchRequest) ([]domain.Event, error)
}

type firestoreRepo struct {
	client *firestore.Client
}

func NewFirestoreRepository(client *firestore.Client) EventRepository {
	return &firestoreRepo{client: client}
}

func (r *firestoreRepo) Save(ctx context.Context, event *domain.Event) error {
	_, err := r.client.Collection(CollectionName).Doc(event.ID).Set(ctx, event)
	return err
}

func (r *firestoreRepo) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	// MergeAll tells Firestore to ONLY update the fields present in the 'updates' map
	_, err := r.client.Collection(CollectionName).Doc(id).Set(ctx, updates, firestore.MergeAll)
	return err
}

func (r *firestoreRepo) GetByID(ctx context.Context, id string) (*domain.Event, error) {
	doc, err := r.client.Collection(CollectionName).Doc(id).Get(ctx)
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
	_, err := r.client.Collection(CollectionName).Doc(id).Delete(ctx)
	return err
}

func (r *firestoreRepo) List(ctx context.Context, search domain.SearchRequest) ([]domain.Event, error) {
	query := r.client.Collection(CollectionName).Query

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
	sortKey := "created_at" // Default
	dir := firestore.Desc   // Default

	if search.Sorting.SortKey != "" {
		sortKey = search.Sorting.SortKey
	}

	if search.Sorting.SortDirection == "asc" {
		dir = firestore.Asc
	}

	query = query.OrderBy(sortKey, dir)

	// 3. Apply Pagination
	pageSize := 10 // Default
	if search.Sorting.PageSize > 0 {
		pageSize = search.Sorting.PageSize
	}

	pageNumber := 1
	if search.Sorting.PageNumber > 1 {
		pageNumber = search.Sorting.PageNumber
	}

	offset := (pageNumber - 1) * pageSize
	query = query.Limit(pageSize).Offset(offset)

	// Execute
	iter := query.Documents(ctx)
	defer iter.Stop()

	events := []domain.Event{}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var e domain.Event
		if err := doc.DataTo(&e); err != nil {
			return nil, fmt.Errorf("failed to map document: %v", err)
		}
		events = append(events, e)
	}

	return events, nil
}
