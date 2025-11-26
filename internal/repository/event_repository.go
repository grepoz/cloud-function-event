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

const CollectionEvents = "events"

type EventRepository interface {
	List(ctx context.Context, search domain.SearchRequest) ([]domain.Event, error)
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*domain.Event, error)
	Update(ctx context.Context, id string, updates map[string]interface{}) error
	Save(ctx context.Context, event *domain.Event) error
}

type eventRepo struct {
	client *firestore.Client
}

func NewEventRepository(client *firestore.Client) EventRepository {
	return &eventRepo{client: client}
}

func (r *eventRepo) Delete(ctx context.Context, id string) error {
	_, err := r.client.Collection(CollectionEvents).Doc(id).Delete(ctx)
	return err
}

func (r *eventRepo) GetByID(ctx context.Context, id string) (*domain.Event, error) {
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

func (r *eventRepo) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	_, err := r.client.Collection(CollectionEvents).Doc(id).Set(ctx, updates, firestore.MergeAll)
	return err
}

func (r *eventRepo) Save(ctx context.Context, event *domain.Event) error {
	_, err := r.client.Collection(CollectionEvents).Doc(event.ID).Set(ctx, event)
	return err
}

func (r *eventRepo) List(ctx context.Context, search domain.SearchRequest) ([]domain.Event, error) {
	q := r.client.Collection(CollectionEvents).Select()
	f := search.Filters

	if f.City != "" {
		q = q.Where("city", "==", f.City)
	}
	if f.Type != "" {
		q = q.Where("type", "==", f.Type)
	}
	if f.MinPrice != nil {
		q = q.Where("price", ">=", *f.MinPrice)
	}
	if f.MaxPrice != nil {
		q = q.Where("price", "<=", *f.MaxPrice)
	}
	if f.StartDate != nil {
		q = q.Where("start_time", ">=", *f.StartDate)
	}
	if f.EndDate != nil {
		q = q.Where("end_time", "<=", *f.EndDate)
	}

	// Sorting
	if search.Sorting.SortKey != "" {
		direction := firestore.Asc
		if search.Sorting.SortDirection == "desc" {
			direction = firestore.Desc
		}
		q = q.OrderBy(search.Sorting.SortKey, direction)
	}

	// Pagination
	pageSize := search.Sorting.PageSize
	if pageSize <= 0 {
		pageSize = 20 // default
	}
	if pageSize > 100 {
		pageSize = 100
	}
	q = q.Limit(pageSize)

	// Cursor-based pagination (if token provided)
	if search.Sorting.PageToken != "" {
		cursor, err := decodeCursor(search.Sorting.PageToken)
		if err == nil && len(cursor) == 2 {
			q = q.StartAfter(cursor...)
		}
	}

	iter := q.Documents(ctx)
	defer iter.Stop()

	var events []domain.Event
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
			continue
		}
		events = append(events, e)
	}

	return events, nil
}

func getSortValue(e *domain.Event, key string) interface{} {
	switch key {
	case "price":
		return e.Price
	case "start_time":
		return e.StartTime
	case "end_time":
		return e.EndTime
	case "city":
		return e.City
	case "type":
		return e.Type
	default:
		return e.ID
	}
}
