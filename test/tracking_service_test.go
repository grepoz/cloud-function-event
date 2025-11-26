package test

import (
	"cloud-function-event/internal/domain"
	"cloud-function-event/internal/service"
	"context"
	"errors"
	"testing"
	"time"
)

// MockTrackingRepo for tracking tests
type MockTrackingRepo struct {
	SaveFunc func(ctx context.Context, t *domain.TrackingEvent) error
	ListFunc func(ctx context.Context) ([]domain.TrackingEvent, error)
}

func (m *MockTrackingRepo) SaveTracking(ctx context.Context, t *domain.TrackingEvent) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, t)
	}
	return nil
}

func (m *MockTrackingRepo) ListTracking(ctx context.Context) ([]domain.TrackingEvent, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

func TestGetAllTracking(t *testing.T) {
	expectedData := []domain.TrackingEvent{
		{ID: "t1", Action: "click", CreatedAt: time.Now()},
		{ID: "t2", Action: "view", CreatedAt: time.Now()},
	}

	mockRepo := &MockTrackingRepo{
		ListFunc: func(ctx context.Context) ([]domain.TrackingEvent, error) {
			return expectedData, nil
		},
	}

	svc := service.NewTrackingService(mockRepo)
	result, err := svc.GetAllTracking(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 items, got %d", len(result))
	}
	if result[0].ID != "t1" {
		t.Errorf("Expected first item ID 't1', got '%s'", result[0].ID)
	}
}

func TestTrackEvent_Validation(t *testing.T) {
	mockRepo := &MockTrackingRepo{}
	svc := service.NewTrackingService(mockRepo)

	// Case 1: Empty Action
	event := &domain.TrackingEvent{Payload: "something"}
	err := svc.TrackEvent(context.Background(), event)
	if err == nil {
		t.Error("Expected validation error for empty action")
	}
}

func TestTrackEvent_Success(t *testing.T) {
	mockRepo := &MockTrackingRepo{
		SaveFunc: func(ctx context.Context, tr *domain.TrackingEvent) error {
			if tr.ID == "" {
				return errors.New("ID was not generated")
			}
			if tr.CreatedAt.IsZero() {
				return errors.New("CreatedAt was not set")
			}
			return nil
		},
	}

	svc := service.NewTrackingService(mockRepo)
	tr := &domain.TrackingEvent{Action: "click", Payload: "data"}
	err := svc.TrackEvent(context.Background(), tr)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}
