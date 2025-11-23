package transport_test

import (
	"cloud-function-event/internal/domain"
	"cloud-function-event/internal/transport"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MockService implements EventService for handler testing
type MockEventService struct {
	CreateFunc func(ctx context.Context, event *domain.Event) error
	UpdateFunc func(ctx context.Context, id string, updates map[string]interface{}) error
	GetFunc    func(ctx context.Context, id string) (*domain.Event, error)
	DeleteFunc func(ctx context.Context, id string) error
	ListFunc   func(ctx context.Context, req domain.SearchRequest) ([]domain.Event, string, error)
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
	// Cel: Sprawdzić czy parametry URL są poprawnie parsowane do SearchRequest
	mockSvc := &MockEventService{
		ListFunc: func(ctx context.Context, req domain.SearchRequest) ([]domain.Event, string, error) {
			// Asercje sprawdzające parsowanie
			if req.Filters.City != "Warsaw" {
				t.Errorf("Expected City 'Warsaw', got '%s'", req.Filters.City)
			}
			if req.Filters.MinPrice == nil || *req.Filters.MinPrice != 50.5 {
				t.Errorf("Expected MinPrice 50.5, got %v", req.Filters.MinPrice)
			}
			if req.Sorting.SortKey != "price" {
				t.Errorf("Expected SortKey 'price', got '%s'", req.Sorting.SortKey)
			}
			if req.Sorting.PageSize != 20 {
				t.Errorf("Expected PageSize 20, got %d", req.Sorting.PageSize)
			}
			return []domain.Event{}, "", nil
		},
	}

	router := transport.NewRouter(mockSvc, &MockTrackingService{})

	req := httptest.NewRequest(http.MethodGet, "/events?city=Warsaw&min_price=50.5&sort_key=price&page_size=20", nil)
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
	req := httptest.NewRequest(http.MethodPost, "/tracking", strings.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Result().StatusCode)
	}
}

func TestHandler_UpdateEvent_UseNumber(t *testing.T) {
	// Cel: Sprawdzić czy JSON number nie jest psuty (czy handler używa UseNumber)
	mockSvc := &MockEventService{
		UpdateFunc: func(ctx context.Context, id string, updates map[string]interface{}) error {
			// Sprawdzamy czy cena jest float64 (bo w JSON 99.99)
			if price, ok := updates["price"].(float64); !ok || price != 99.99 {
				t.Errorf("Expected price 99.99 (float64), got %v (%T)", updates["price"], updates["price"])
			}
			// Sprawdzamy czy capacity jest float64 (domyślne zachowanie po konwersji z json.Number w handlerze)
			// W handlerze robiliśmy rzutowanie na float64 dla json.Number, więc tu oczekujemy float64
			if capVal, ok := updates["capacity"].(float64); !ok || capVal != 100 {
				t.Errorf("Expected capacity 100 (float64), got %v (%T)", updates["capacity"], updates["capacity"])
			}
			return nil
		},
	}
	router := transport.NewRouter(mockSvc, &MockTrackingService{})

	body := `{"price": 99.99, "capacity": 100}`
	req := httptest.NewRequest(http.MethodPut, "/events?id=123", strings.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Result().StatusCode)
	}
}
