package service_test

import (
	"cloud-function-event/internal/domain"
	"cloud-function-event/internal/service"
	"context"
	"errors"
	"reflect"
	"testing"
)

// MockRepository manually implements Repository for testing
type MockRepository struct {
	SaveFunc    func(ctx context.Context, event *domain.Event) error
	UpdateFunc  func(ctx context.Context, id string, updates map[string]interface{}) error
	GetByIDFunc func(ctx context.Context, id string) (*domain.Event, error)
	DeleteFunc  func(ctx context.Context, id string) error
	ListFunc    func(ctx context.Context, search domain.SearchRequest) ([]domain.Event, string, error)
}

func (m *MockRepository) Save(ctx context.Context, event *domain.Event) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, event)
	}
	return nil
}

func (m *MockRepository) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, id, updates)
	}
	return nil
}

func (m *MockRepository) GetByID(ctx context.Context, id string) (*domain.Event, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockRepository) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

func (m *MockRepository) List(ctx context.Context, search domain.SearchRequest) ([]domain.Event, string, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, search)
	}
	return nil, "", nil
}

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
		"eventname": "Updated Meetup",
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
	// Test sprawdza, czy serwis przycina PageSize do 100
	mockRepo := &MockRepository{
		ListFunc: func(ctx context.Context, search domain.SearchRequest) ([]domain.Event, string, error) {
			if search.Sorting.PageSize != 100 {
				t.Errorf("Expected PageSize to be capped at 100, got %d", search.Sorting.PageSize)
			}
			return []domain.Event{}, "next-token", nil
		},
	}

	svc := service.NewEventService(mockRepo)

	// Żądamy 500 elementów
	req := domain.SearchRequest{
		Sorting: domain.SortRequest{PageSize: 500},
	}

	_, token, err := svc.ListEvents(context.Background(), req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if token != "next-token" {
		t.Errorf("Expected token 'next-token', got %s", token)
	}
}
