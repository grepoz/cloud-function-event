package service_test

import (
	"cloud-function-event/internal/domain"
	"cloud-function-event/internal/service"
	"context"
	"errors"
	"testing"
)

// MockRepository manually implements Repository for testing
type MockRepository struct {
	SaveFunc    func(ctx context.Context, event *domain.Event) error
	UpdateFunc  func(ctx context.Context, id string, updates map[string]interface{}) error
	GetByIDFunc func(ctx context.Context, id string) (*domain.Event, error)
	DeleteFunc  func(ctx context.Context, id string) error
	// ZMIANA: Dodano string (nextToken) do sygnatury funkcji
	ListFunc func(ctx context.Context, search domain.SearchRequest) ([]domain.Event, string, error)
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

// ZMIANA: Metoda List musi zwracać ([]domain.Event, string, error)
func (m *MockRepository) List(ctx context.Context, search domain.SearchRequest) ([]domain.Event, string, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, search)
	}
	// Zwracamy pusty string jako token, jeśli funkcja nie jest zdefiniowana
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

func TestGetEventValidation(t *testing.T) {
	svc := service.NewEventService(&MockRepository{})
	_, err := svc.GetEvent(context.Background(), "")

	if err == nil {
		t.Error("Expected error for empty ID")
	}
}
