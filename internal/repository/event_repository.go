package repository

import (
	"cloud-function-event/internal/domain"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const CollectionEvents = "events"

type EventRepository interface {
	List(ctx context.Context, search domain.SearchRequest) ([]domain.Event, string, error)
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

func (r *eventRepo) List(ctx context.Context, search domain.SearchRequest) ([]domain.Event, string, error) {
	// 1. Determine Sorting FIRST
	sortKey := search.Sorting.SortKey
	if sortKey == "" {
		sortKey = "created_at"
	}
	direction := firestore.Asc
	if search.Sorting.SortDirection == "desc" {
		direction = firestore.Desc
	}

	// 2. Initialize 'q' using the OrderBy clause
	// This converts the CollectionRef to a Query immediately
	q := r.client.Collection(CollectionEvents).OrderBy(sortKey, direction)

	f := search.Filters

	// 3. Apply Filters

	lastUtf8Char := "\uf8ff"

	if f.EventName != "" {
		q = q.Where("event_name", ">=", f.EventName)
		q = q.Where("event_name", "<=", f.EventName+lastUtf8Char)
	}
	if f.City != "" {
		q = q.Where("city", ">=", f.City)
		q = q.Where("city", "<=", f.City+lastUtf8Char)
	}
	// TODO - make enums for Type
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

	// 4. Apply Secondary Sort (ID)
	// We append this to the existing sort order
	q = q.OrderBy("id", firestore.Asc)

	// 5. Pagination Limit
	limit := search.Sorting.PageSize
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	q = q.Limit(limit)

	// Handle Page Token (Cursor)
	if search.Sorting.PageToken != "" {
		cursorVals, err := decodeCursor(search.Sorting.PageToken)
		if err != nil {
			return nil, "", fmt.Errorf("invalid page token")
		}

		if len(cursorVals) > 0 {
			switch sortKey {
			case "created_at", "start_time", "end_time":
				// Check if it's a string and parse it
				if strVal, ok := cursorVals[0].(string); ok {
					t, err := time.Parse(time.RFC3339, strVal)
					if err == nil {
						cursorVals[0] = t
					}
				}
			}
		}

		q = q.StartAfter(cursorVals...)
	}

	// Execute Query
	iter := q.Documents(ctx)
	defer iter.Stop()

	var events []domain.Event
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, "", err
		}

		var e domain.Event
		if err := doc.DataTo(&e); err != nil {
			return nil, "", err
		}

		events = append(events, e)
	}

	// Generate Next Page Token
	nextToken := ""
	if len(events) == limit {
		lastEvent := events[len(events)-1]
		// We need to encode values for all OrderBy fields: [sortKey, id]
		val := getSortValue(&lastEvent, sortKey)
		nextToken = encodeCursor([]interface{}{val, lastEvent.ID})
	}

	return events, nextToken, nil
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
	case "created_at":
		return e.CreatedAt
	case "eventname":
		return e.EventName
	default:
		return e.CreatedAt
	}
}

func encodeCursor(vals []interface{}) string {
	b, _ := json.Marshal(vals)
	return base64.StdEncoding.EncodeToString(b)
}

func decodeCursor(token string) ([]interface{}, error) {
	b, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	var vals []interface{}
	err = json.Unmarshal(b, &vals)
	return vals, err
}
