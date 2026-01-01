package unit_tests

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bibently.com/backend/internal/domain"
	"bibently.com/backend/internal/transport"
)

type MockEventService struct {
	CreateFunc      func(ctx context.Context, event *domain.Event) error
	BatchCreateFunc func(ctx context.Context, events []*domain.Event) error
	UpdateFunc      func(ctx context.Context, id string, updates map[string]interface{}) error
	GetFunc         func(ctx context.Context, id string) (*domain.Event, error)
	DeleteFunc      func(ctx context.Context, id string) error
	ListFunc        func(ctx context.Context, req domain.SearchRequest) ([]domain.Event, string, error)
}

func (m *MockEventService) CreateEvent(ctx context.Context, event *domain.Event) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, event)
	}
	return nil
}
func (m *MockEventService) UpdateEvent(ctx context.Context, id string, updates map[string]interface{}) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, id, updates)
	}
	return nil
}
func (m *MockEventService) GetEvent(ctx context.Context, id string) (*domain.Event, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, id)
	}
	return nil, nil
}
func (m *MockEventService) DeleteEvent(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}
func (m *MockEventService) ListEvents(ctx context.Context, req domain.SearchRequest) ([]domain.Event, string, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, req)
	}
	return nil, "", nil
}
func (m *MockEventService) BatchCreateEvents(ctx context.Context, events []*domain.Event) error {
	if m.BatchCreateFunc != nil {
		return m.BatchCreateFunc(ctx, events)
	}
	return nil
}

type MockTrackingService struct {
	TrackFunc  func(ctx context.Context, event *domain.TrackingEvent) error
	GetAllFunc func(ctx context.Context) ([]domain.TrackingEvent, error)
}

func (m *MockTrackingService) TrackEvent(ctx context.Context, event *domain.TrackingEvent) error {
	if m.TrackFunc != nil {
		return m.TrackFunc(ctx, event)
	}
	return nil
}
func (m *MockTrackingService) GetAllTracking(ctx context.Context) ([]domain.TrackingEvent, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc(ctx)
	}
	return nil, nil
}

func TestHandler_ListEvents_QueryParams(t *testing.T) {
	mockSvc := &MockEventService{
		ListFunc: func(ctx context.Context, req domain.SearchRequest) ([]domain.Event, string, error) {
			if req.Filters.City != "Warsaw" {
				t.Errorf("Expected City 'Warsaw', got '%s'", req.Filters.City)
			}
			if req.Filters.MinPrice == nil || *req.Filters.MinPrice != 50.5 {
				t.Errorf("Expected MinPrice 50.5, got %v", req.Filters.MinPrice)
			}
			if len(req.Filters.Keywords) != 2 {
				t.Errorf("Expected 2 keywords, got %d", len(req.Filters.Keywords))
			}
			if req.Filters.Keywords[0] != "music" || req.Filters.Keywords[1] != "art" {
				t.Errorf("Expected keywords [music, art], got %v", req.Filters.Keywords)
			}

			return []domain.Event{}, "", nil
		},
	}

	router := transport.NewRouter(mockSvc, &MockTrackingService{})

	req := httptest.NewRequest(http.MethodGet, "/events/?city=Warsaw&min_price=50.5&keywords=music,art", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Result().StatusCode)
	}
}

func TestTrackingHandler_Create(t *testing.T) {
	mockTrack := &MockTrackingService{
		TrackFunc: func(ctx context.Context, event *domain.TrackingEvent) error {
			if event.Action != "login" {
				t.Errorf("Expected action 'login', got '%s'", event.Action)
			}
			return nil
		},
	}
	router := transport.NewRouter(&MockEventService{}, mockTrack)
	body := `{"action": "login", "payload": "user_123"}`
	req := httptest.NewRequest(http.MethodPost, "/tracking/", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Result().StatusCode)
	}
}

func TestHandler_UpdateEvent_Success(t *testing.T) {
	mockSvc := &MockEventService{
		UpdateFunc: func(ctx context.Context, id string, updates map[string]interface{}) error {
			if id != "123" {
				t.Errorf("Expected id '123', got '%s'", id)
			}
			// name should be string
			if name, ok := updates["name"].(string); !ok || name != "New Name" {
				t.Errorf("Expected name 'New Name', got %v", updates["name"])
			}
			return nil
		},
	}
	router := transport.NewRouter(mockSvc, &MockTrackingService{})

	body := `{"name": "New Name"}`
	req := httptest.NewRequest(http.MethodPut, "/events/123", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Result().StatusCode)
	}
}

// to do - add test for nested prop update

func TestHandler_UpdateEvent_Security_MassAssignment(t *testing.T) {
	mockSvc := &MockEventService{
		UpdateFunc: func(ctx context.Context, id string, updates map[string]interface{}) error {
			if _, exists := updates["is_admin"]; exists {
				t.Error("Security Fail: 'is_admin' field was passed to service")
			}
			if val, ok := updates["name"]; !ok || val != "Hacked Name" {
				t.Error("Expected 'name' to be updated")
			}
			return nil
		},
	}
	router := transport.NewRouter(mockSvc, &MockTrackingService{})
	body := `{"name": "Hacked Name", "is_admin": true}`
	req := httptest.NewRequest(http.MethodPut, "/events/123", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}
}

// TestHandler_UpdateEvent_TypePollution verifies that wrong types cause 400 Bad Request
func TestHandler_UpdateEvent_TypePollution(t *testing.T) {
	mockSvc := &MockEventService{
		UpdateFunc: func(ctx context.Context, id string, updates map[string]interface{}) error {
			t.Error("Service should NOT be called for type pollution")
			return nil
		},
	}
	router := transport.NewRouter(mockSvc, &MockTrackingService{})

	// "price" expects number, we send string
	body := `{"offer.price": "free"}`
	req := httptest.NewRequest(http.MethodPut, "/events/123", strings.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for type mismatch, got %d", w.Code)
	}
}

// TestHandler_UpdateEvent_Validation verifies DTO validation rules (e.g. gte=0)
func TestHandler_UpdateEvent_Validation(t *testing.T) {
	mockSvc := &MockEventService{
		UpdateFunc: func(ctx context.Context, id string, updates map[string]interface{}) error {
			t.Error("Service should NOT be called for validation error")
			return nil
		},
	}
	router := transport.NewRouter(mockSvc, &MockTrackingService{})

	// "price" must be >= 0
	body := `{"offer.price": -50.00}`
	req := httptest.NewRequest(http.MethodPut, "/events/123", strings.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for validation error, got %d", w.Code)
	}
}

func TestEventHandler_Create_InvalidJSON(t *testing.T) {
	router := transport.NewRouter(&MockEventService{}, &MockTrackingService{})
	body := `{"name": "Broken JSON"`
	req := httptest.NewRequest(http.MethodPost, "/events/", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid JSON, got %d", w.Code)
	}
}

func TestEventHandler_Get_NotFound(t *testing.T) {
	mockSvc := &MockEventService{
		GetFunc: func(ctx context.Context, id string) (*domain.Event, error) {
			return nil, errors.New("event not found")
		},
	}
	router := transport.NewRouter(mockSvc, &MockTrackingService{})
	req := httptest.NewRequest(http.MethodGet, "/events/missing-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", w.Code)
	}
}
