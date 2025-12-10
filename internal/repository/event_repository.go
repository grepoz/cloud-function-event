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
	_, err := r.client.Collection(CollectionEvents).Doc(event.Id).Set(ctx, event)
	return err
}

func (r *eventRepo) List(ctx context.Context, search domain.SearchRequest) ([]domain.Event, string, error) {

	validSorts := map[string]bool{
		"created_at": true, "price": true, "start_time": true,
		"event_name": true, "city": true, "end_time": true,
	}

	f := search.Filters
	reqSort := search.Sorting.SortKey

	// 1. Identify active inequality filters
	// Firestore requires that if you filter by inequality (range) on multiple fields,
	// you must also order by those fields in the query to utilize the index efficiently.
	var inequalityFields []string

	// Prefix matches (>= and <=) count as inequalities
	if f.EventName != "" {
		inequalityFields = append(inequalityFields, "event_name")
	}
	if f.City != "" {
		inequalityFields = append(inequalityFields, "city")
	}
	// Numeric and Date ranges
	if f.MinPrice != nil || f.MaxPrice != nil {
		inequalityFields = append(inequalityFields, "price")
	}
	if f.StartDate != nil || f.EndDate != nil {
		inequalityFields = append(inequalityFields, "start_time")
	}

	// 2. Build Sort Order
	var sortFields []string

	// A. Add all inequality fields to sort first (Critical for Firestore logic)
	for _, field := range inequalityFields {
		sortFields = append(sortFields, field)
	}

	// B. Add User's requested sort (if not already added via inequality)
	if reqSort != "" && validSorts[reqSort] {
		isDuplicate := false
		for _, existing := range sortFields {
			if existing == reqSort {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			sortFields = append(sortFields, reqSort)
		}
	}

	// C. Fallback: If no sorts yet, default to created_at
	if len(sortFields) == 0 {
		sortFields = append(sortFields, "created_at")
	}

	// D. Always tie-break with ID for stable pagination
	sortFields = append(sortFields, "id")

	// 3. Build Query (Apply Sorts)
	coll := r.client.Collection(CollectionEvents)
	var q firestore.Query

	direction := firestore.Asc
	if search.Sorting.SortDirection == "desc" {
		direction = firestore.Desc
	}

	for i, field := range sortFields {
		// Calculate direction for this specific field
		dir := direction
		if field == "id" {
			dir = firestore.Asc // ID is always Ascending for stability
		}

		// First iteration applies to CollectionRef and returns a Query
		// Subsequent iterations apply to Query and return a Query
		if i == 0 {
			q = coll.OrderBy(field, dir)
		} else {
			q = q.OrderBy(field, dir)
		}
	}

	// 4. Apply Filters
	lastUtf8Char := "\uf8ff"

	if f.EventName != "" {
		q = q.Where("event_name", ">=", f.EventName).Where("event_name", "<=", f.EventName+lastUtf8Char)
	}
	if f.City != "" {
		q = q.Where("city", ">=", f.City).Where("city", "<=", f.City+lastUtf8Char)
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

	// 5. Pagination Limit
	limit := search.Sorting.PageSize
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	q = q.Limit(limit)

	// 6. Handle Page Token (Cursor)
	if search.Sorting.PageToken != "" {
		cursorVals, err := decodeCursor(search.Sorting.PageToken)
		if err != nil {
			return nil, "", fmt.Errorf("invalid page token")
		}

		// Safety Check: Cursor length must match the number of OrderBy fields
		if len(cursorVals) != len(sortFields) {
			return nil, "", fmt.Errorf("cursor mismatch: sorting criteria changed")
		}

		// Correctly parse time strings based on the field type in that position
		for i, field := range sortFields {
			switch field {
			case "created_at", "start_time", "end_time":
				if strVal, ok := cursorVals[i].(string); ok {
					t, err := time.Parse(time.RFC3339, strVal)
					if err == nil {
						cursorVals[i] = t
					}
				}
			}
		}

		q = q.StartAfter(cursorVals...)
	}

	// 7. Execute Query
	iter := q.Documents(ctx)
	defer iter.Stop()

	var events []domain.Event
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
			return nil, "", err
		}

		events = append(events, e)
	}

	// 8. Generate Next Page Token
	nextToken := ""
	if len(events) == limit {
		lastEvent := events[len(events)-1]

		var cursorValues []interface{}
		// Generate cursor values exactly matching the sortFields list
		for _, field := range sortFields {
			if field == "id" {
				cursorValues = append(cursorValues, lastEvent.Id)
			} else {
				cursorValues = append(cursorValues, getSortValue(&lastEvent, field))
			}
		}

		nextToken = encodeCursor(cursorValues)
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
	case "event_name":
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
