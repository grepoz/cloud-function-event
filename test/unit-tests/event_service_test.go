package unit_tests

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"bibently.com/backend/internal/domain"
	"bibently.com/backend/internal/service"
	"bibently.com/backend/test"
)

func TestCreateEvent(t *testing.T) {
	mockRepo := &test.MockRepository{
		SaveFunc: func(ctx context.Context, event *domain.Event) error {
			if event.Id == "" {
				return errors.New("id was not generated")
			}
			return nil
		},
	}

	svc := service.NewEventService(mockRepo)
	event := &domain.Event{Name: "Go Meetup"} // Field name updated

	err := svc.CreateEvent(context.Background(), event)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if event.Id == "" {
		t.Error("Expected Id to be generated")
	}
}

func TestCreateEvent_Validation(t *testing.T) {
	mockRepo := &test.MockRepository{}
	svc := service.NewEventService(mockRepo)

	// Case: Empty Name
	event := &domain.Event{
		Location: domain.Location{Address: domain.Address{City: "Warsaw"}},
		// Name is missing
	}

	err := svc.CreateEvent(context.Background(), event)
	if err == nil {
		t.Error("Expected validation error for empty Name, got nil")
	}
	// Note: validation string might need update depending on service logic update
	// Assuming service checks event.Name now
}

func TestBatchCreateEvents(t *testing.T) {
	mockRepo := &test.MockRepository{
		BatchSaveFunc: func(ctx context.Context, events []*domain.Event) error {
			if len(events) != 2 {
				t.Errorf("Expected 2 events in batch, got %d", len(events))
			}
			for _, e := range events {
				if e.Id == "" {
					return errors.New("ID not generated")
				}
				if e.CreatedAt.IsZero() {
					return errors.New("CreatedAt not set")
				}
			}
			return nil
		},
	}

	svc := service.NewEventService(mockRepo)

	events := []*domain.Event{
		{Name: "Event 1", Location: domain.Location{Address: domain.Address{City: "Warsaw"}}},
		{Name: "Event 2", Location: domain.Location{Address: domain.Address{City: "Krakow"}}},
	}

	err := svc.BatchCreateEvents(context.Background(), events)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestUpdateEvent(t *testing.T) {
	mockRepo := &test.MockRepository{
		UpdateFunc: func(ctx context.Context, id string, updates map[string]interface{}) error {
			return nil
		},
	}

	svc := service.NewEventService(mockRepo)

	err := svc.UpdateEvent(context.Background(), "", map[string]interface{}{"name": "test"})
	if err == nil {
		t.Error("Expected error for missing Id on update")
	}

	err = svc.UpdateEvent(context.Background(), "123", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for no fields to update")
	}

	updates := map[string]interface{}{
		"name": "Updated Meetup",
	}
	err = svc.UpdateEvent(context.Background(), "123", updates)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestGetEvent(t *testing.T) {
	expectedEvent := &domain.Event{Id: "123", Name: "Test Event"}
	mockRepo := &test.MockRepository{
		GetByIDFunc: func(ctx context.Context, id string) (*domain.Event, error) {
			if id == "123" {
				return expectedEvent, nil
			}
			return nil, errors.New("not found")
		},
	}

	svc := service.NewEventService(mockRepo)

	_, err := svc.GetEvent(context.Background(), "")
	if err == nil {
		t.Error("Expected error for empty Id")
	}

	event, err := svc.GetEvent(context.Background(), "123")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !reflect.DeepEqual(event, expectedEvent) {
		t.Errorf("Expected event %v, got %v", expectedEvent, event)
	}
}

func TestDeleteEvent(t *testing.T) {
	mockRepo := &test.MockRepository{
		DeleteFunc: func(ctx context.Context, id string) error {
			if id == "valid" {
				return nil
			}
			return errors.New("db error")
		},
	}
	svc := service.NewEventService(mockRepo)

	if err := svc.DeleteEvent(context.Background(), ""); err == nil {
		t.Error("Expected error for empty Id")
	}

	if err := svc.DeleteEvent(context.Background(), "valid"); err != nil {
		t.Errorf("Expected success, got %v", err)
	}
}

func TestListEvents_PageSizeCap(t *testing.T) {
	mockRepo := &test.MockRepository{
		ListFunc: func(ctx context.Context, search domain.SearchRequest) ([]domain.Event, string, error) {
			if search.Sorting.PageSize != 100 {
				t.Errorf("Expected PageSize to be capped at 100, got %d", search.Sorting.PageSize)
			}
			return []domain.Event{}, "", nil
		},
	}

	svc := service.NewEventService(mockRepo)

	req := domain.SearchRequest{
		Sorting: domain.SortRequest{PageSize: 500},
	}

	_, _, err := svc.ListEvents(context.Background(), req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}
