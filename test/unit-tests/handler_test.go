package unit_tests

import (
	"bibently.com/backend/internal/domain"
	"bibently.com/backend/internal/transport"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MockService implements EventService for handler testing
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
			return []domain.Event{}, "", nil
		},
	}

	router := transport.NewRouter(mockSvc, &MockTrackingService{})

	// Note: trailing slash required for collection root in standard mux if registered as /events/
	req := httptest.NewRequest(http.MethodGet, "/events/?city=Warsaw&min_price=50.5", nil)
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
	// Note: trailing slash
	req := httptest.NewRequest(http.MethodPost, "/tracking/", strings.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Result().StatusCode)
	}
}

// TestHandler_UpdateEvent_Success validates the happy path using the new DTO logic
func TestHandler_UpdateEvent_Success(t *testing.T) {
	mockSvc := &MockEventService{
		UpdateFunc: func(ctx context.Context, id string, updates map[string]interface{}) error {
			if id != "123" {
				t.Errorf("Expected id '123', got '%s'", id)
			}
			// The DTO now guarantees this is a *float64 which dereferences to float64
			if price, ok := updates["price"].(float64); !ok || price != 99.99 {
				t.Errorf("Expected price 99.99 (float64), got %v (type %T)", updates["price"], updates["price"])
			}
			return nil
		},
	}
	router := transport.NewRouter(mockSvc, &MockTrackingService{})

	body := `{"price": 99.99}`
	// URL uses Path Parameter now: /events/123
	req := httptest.NewRequest(http.MethodPut, "/events/123", strings.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Result().StatusCode)
	}
}

// TestHandler_UpdateEvent_Security_MassAssignment verifies that injected fields are ignored
func TestHandler_UpdateEvent_Security_MassAssignment(t *testing.T) {
	mockSvc := &MockEventService{
		UpdateFunc: func(ctx context.Context, id string, updates map[string]interface{}) error {
			// 1. Verify injected fields are NOT present
			if _, exists := updates["is_admin"]; exists {
				t.Error("Security Fail: 'is_admin' field was passed to service")
			}
			if _, exists := updates["created_at"]; exists {
				t.Error("Security Fail: 'created_at' field was passed to service")
			}

			// 2. Verify valid fields ARE present
			if val, ok := updates["event_name"]; !ok || val != "Hacked Name" {
				t.Error("Expected 'event_name' to be updated")
			}
			return nil
		},
	}
	router := transport.NewRouter(mockSvc, &MockTrackingService{})

	// Malicious Payload: includes valid field + ignored fields
	body := `{"event_name": "Hacked Name", "is_admin": true, "created_at": "2020-01-01"}`
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
	body := `{"price": "free"}`
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
	body := `{"price": -50.00}`
	req := httptest.NewRequest(http.MethodPut, "/events/123", strings.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for validation error, got %d", w.Code)
	}
}

func TestEventHandler_Create_InvalidJSON(t *testing.T) {
	router := transport.NewRouter(&MockEventService{}, &MockTrackingService{})

	// Send invalid JSON (missing closing brace)
	body := `{"event_name": "Broken JSON"`
	req := httptest.NewRequest(http.MethodPost, "/events/", strings.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid JSON, got %d", w.Code)
	}
}

func TestEventHandler_Get_NotFound(t *testing.T) {
	// Mock service to return an error looking like "event not found"
	mockSvc := &MockEventService{
		GetFunc: func(ctx context.Context, id string) (*domain.Event, error) {
			// Matches the error check in transport/handler.go: respondError
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

func TestTrackingHandler_List(t *testing.T) {
	expectedTracks := []domain.TrackingEvent{
		{Id: "track_1", Action: "signup"},
	}

	mockTrackSvc := &MockTrackingService{
		GetAllFunc: func(ctx context.Context) ([]domain.TrackingEvent, error) {
			return expectedTracks, nil
		},
	}

	router := transport.NewRouter(&MockEventService{}, mockTrackSvc)

	req := httptest.NewRequest(http.MethodGet, "/tracking/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}

	// Verify JSON response
	var resp domain.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal("Failed to decode response")
	}

	// Cast 'data' (interface{}) back to slice to verify content
	// Note: JSON unmarshaling into interface{} results in map[string]interface{} for objects
	dataSlice, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("Response data is not a slice, got %T", resp.Data)
	}

	if len(dataSlice) != 1 {
		t.Errorf("Expected 1 tracking event, got %d", len(dataSlice))
	}

	item := dataSlice[0].(map[string]interface{})
	if item["Action"] != "signup" {
		t.Errorf("Expected action 'signup', got %v", item["action"])
	}
}
