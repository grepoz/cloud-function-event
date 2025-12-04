package test

import (
	"cloud-function-event/internal/domain"
	"cloud-function-event/internal/service"
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestCreateEvent(t *testing.T) {
	mockRepo := &MockRepository{
		SaveFunc: func(ctx context.Context, event *domain.Event) error {
			if event.ID == "" {
				return errors.New("ID was not generated")
			}
			return nil
		},
	}

	svc := service.NewEventService(mockRepo)
	event := &domain.Event{EventName: "Go Meetup"}

	err := svc.CreateEvent(context.Background(), event)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if event.ID == "" {
		t.Error("Expected ID to be generated")
	}
}

func TestCreateEvent_Validation(t *testing.T) {
	mockRepo := &MockRepository{} // No methods needed, should fail before repo call
	svc := service.NewEventService(mockRepo)

	// Case: Empty EventName
	event := &domain.Event{
		City: "Warsaw",
		// EventName is missing
	}

	err := svc.CreateEvent(context.Background(), event)
	if err == nil {
		t.Error("Expected validation error for empty EventName, got nil")
	}

	expectedErr := "event name is required"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestUpdateEvent(t *testing.T) {
	mockRepo := &MockRepository{
		UpdateFunc: func(ctx context.Context, id string, updates map[string]interface{}) error {
			return nil
		},
	}

	svc := service.NewEventService(mockRepo)

	// Case 1: Missing ID
	err := svc.UpdateEvent(context.Background(), "", map[string]interface{}{"name": "test"})
	if err == nil {
		t.Error("Expected error for missing ID on update")
	}

	// Case 2: Empty Updates
	err = svc.UpdateEvent(context.Background(), "123", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for no fields to update")
	}

	// Case 3: Valid Update
	updates := map[string]interface{}{
		"event_name": "Updated Meetup",
	}
	err = svc.UpdateEvent(context.Background(), "123", updates)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestGetEvent(t *testing.T) {
	expectedEvent := &domain.Event{ID: "123", EventName: "Test Event"}
	mockRepo := &MockRepository{
		GetByIDFunc: func(ctx context.Context, id string) (*domain.Event, error) {
			if id == "123" {
				return expectedEvent, nil
			}
			return nil, errors.New("not found")
		},
	}

	svc := service.NewEventService(mockRepo)

	// Case 1: Validation
	_, err := svc.GetEvent(context.Background(), "")
	if err == nil {
		t.Error("Expected error for empty ID")
	}

	// Case 2: Success
	event, err := svc.GetEvent(context.Background(), "123")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !reflect.DeepEqual(event, expectedEvent) {
		t.Errorf("Expected event %v, got %v", expectedEvent, event)
	}
}

func TestDeleteEvent(t *testing.T) {
	mockRepo := &MockRepository{
		DeleteFunc: func(ctx context.Context, id string) error {
			if id == "valid" {
				return nil
			}
			return errors.New("db error")
		},
	}
	svc := service.NewEventService(mockRepo)

	if err := svc.DeleteEvent(context.Background(), ""); err == nil {
		t.Error("Expected error for empty ID")
	}

	if err := svc.DeleteEvent(context.Background(), "valid"); err != nil {
		t.Errorf("Expected success, got %v", err)
	}
}

func TestListEvents_PageSizeCap(t *testing.T) {
	mockRepo := &MockRepository{
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
