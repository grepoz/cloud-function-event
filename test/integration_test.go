package test

import (
	"bytes"
	"cloud-function-event/internal/domain"
	"cloud-function-event/internal/repository"
	"cloud-function-event/internal/service"
	"cloud-function-event/internal/transport"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
)

func setupIntegration(t *testing.T) (http.Handler, *firestore.Client) {

	err := os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080")
	if err != nil {
		t.Fatalf("Failed to set FIRESTORE_EMULATOR_HOST: %v", err)
		return nil, nil
	}

	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		t.Skip("Skipping integration test: FIRESTORE_EMULATOR_HOST not set")
	}

	ctx := context.Background()
	projectID := "local-project-id"

	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to create firestore client: %v", err)
	}

	eventRepo := repository.NewEventRepository(client)
	trackingRepo := repository.NewTrackingRepository(client)
	eventSvc := service.NewEventService(eventRepo)
	trackingSvc := service.NewTrackingService(trackingRepo)

	router := transport.NewRouter(eventSvc, trackingSvc)

	return router, client
}
func TestIntegration_Tracking(t *testing.T) {
	router, client := setupIntegration(t)
	defer client.Close()

	trackPayload := map[string]string{
		"action":  "button_click",
		"payload": "signup_page",
	}
	body, _ := json.Marshal(trackPayload)

	req := httptest.NewRequest(http.MethodPost, "/tracking/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected 201 Created, got %d", w.Code)
	}

	reqGet := httptest.NewRequest(http.MethodGet, "/tracking/", nil)
	wGet := httptest.NewRecorder()

	router.ServeHTTP(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", wGet.Code)
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(wGet.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	dataBytes, _ := json.Marshal(resp.Data)
	if !bytes.Contains(dataBytes, []byte("button_click")) {
		t.Errorf("Response did not contain tracked action. Got: %s", string(dataBytes))
	}
}

func TestIntegration_CreateAndGetEvent(t *testing.T) {
	router, client := setupIntegration(t)
	defer client.Close()

	newEvent := map[string]interface{}{
		"eventname": "Integration Concert",
		"city":      "Warsaw",
		"type":      "concert",
	}
	body, _ := json.Marshal(newEvent)

	// Use trailing slash for collection
	req := httptest.NewRequest(http.MethodPost, "/events/", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp domain.APIResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	eventID := resp.Data.(string)

	// --- Use Path Parameter for GET ---
	reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/events/%s", eventID), nil)
	wGet := httptest.NewRecorder()

	router.ServeHTTP(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", wGet.Code)
	}

	var respGet domain.APIResponse
	// Dekodujemy do mapy, bo domain.Event ma typy czasowe, które JSON unmarshaluje różnie
	if err := json.NewDecoder(wGet.Body).Decode(&respGet); err != nil {
		t.Fatalf("Failed to decode GET response: %v", err)
	}

	// Konwersja mapy z powrotem na struct lub asercja na mapie
	// Tutaj użyjemy prostego sprawdzenia JSON
	respJSON, _ := json.Marshal(respGet.Data)
	if !bytes.Contains(respJSON, []byte("Integration Concert")) {
		t.Errorf("Response does not contain event name. Got: %s", string(respJSON))
	}
}

func TestIntegration_ListEvents(t *testing.T) {
	router, client := setupIntegration(t)
	defer client.Close()

	// 1. Setup: Create an event so the list is not empty
	// IMPORTANT: Use trailing slash "/events/" for POST
	createBody := `{"eventname":"List Me", "city":"Cracow", "price": 50}`
	createReq := httptest.NewRequest(http.MethodPost, "/events/", bytes.NewReader([]byte(createBody)))
	wCreate := httptest.NewRecorder()
	router.ServeHTTP(wCreate, createReq)

	if wCreate.Code != http.StatusCreated {
		t.Fatalf("Setup failed: Expected 201 Created, got %d", wCreate.Code)
	}

	// 2. Test: List events (GET)
	req := httptest.NewRequest(http.MethodGet, "/events/?city=Cracow", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w.Code)
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	dataSlice, ok := resp.Data.([]interface{})
	if !ok {
		t.Logf("Data type is %T", resp.Data)
	} else {
		if len(dataSlice) == 0 {
			t.Error("Expected non-empty list of events")
		}
	}
}

func TestIntegration_UpdateAndDelete(t *testing.T) {
	router, client := setupIntegration(t)
	defer client.Close()

	// Create
	createBody := `{"eventname": "To Change", "city": "Old City"}`
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/events/", bytes.NewReader([]byte(createBody))))

	var resp domain.APIResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	eventID := resp.Data.(string)

	// Update (PUT with Path Param)
	updateBody := `{"city": "New City"}`
	reqUpdate := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/events/%s", eventID), bytes.NewReader([]byte(updateBody)))
	wUpdate := httptest.NewRecorder()
	router.ServeHTTP(wUpdate, reqUpdate)

	if wUpdate.Code != http.StatusOK {
		t.Fatalf("Update failed, got %d", wUpdate.Code)
	}

	// Delete (DELETE with Path Param)
	reqDel := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/events/%s", eventID), nil)
	wDel := httptest.NewRecorder()
	router.ServeHTTP(wDel, reqDel)

	if wDel.Code != http.StatusOK {
		t.Errorf("Delete failed, got %d", wDel.Code)
	}
}

func TestIntegration_Pagination(t *testing.T) {
	router, client := setupIntegration(t)
	defer client.Close()

	// 1. Prepare data
	titles := []string{"Page1_A", "Page1_B", "Page2_A", "Page2_B"}
	for _, title := range titles {
		body := fmt.Sprintf(`{"eventname": "%s", "city": "PaginationTest", "start_time": "%s"}`,
			title, time.Now().Add(time.Hour).Format(time.RFC3339))

		// FIX: Use "/events/" (with trailing slash)
		req := httptest.NewRequest(http.MethodPost, "/events/", bytes.NewReader([]byte(body)))
		router.ServeHTTP(httptest.NewRecorder(), req)
	}

	// 2. Get Page 1
	reqPage1 := httptest.NewRequest(http.MethodGet, "/events/?city=PaginationTest&page_size=2", nil)
	wPage1 := httptest.NewRecorder()
	router.ServeHTTP(wPage1, reqPage1)

	var resp1 domain.APIResponse
	if err := json.NewDecoder(wPage1.Body).Decode(&resp1); err != nil {
		t.Fatal(err)
	}

	data1 := resp1.Data.([]interface{})
	if len(data1) != 2 {
		t.Fatalf("Expected 2 events on page 1, got %d", len(data1))
	}

	token := resp1.Meta.NextPageToken
	if token == "" {
		t.Fatal("Expected NextPageToken in response")
	}

	// 3. Get Page 2
	reqPage2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/events/?city=PaginationTest&page_size=2&page_token=%s", token), nil)
	wPage2 := httptest.NewRecorder()
	router.ServeHTTP(wPage2, reqPage2)

	var resp2 domain.APIResponse
	if err := json.NewDecoder(wPage2.Body).Decode(&resp2); err != nil {
		t.Fatal(err)
	}

	data2 := resp2.Data.([]interface{})
	if len(data2) != 2 {
		t.Fatalf("Expected 2 events on page 2, got %d", len(data2))
	}
}

func TestIntegration_ComplexFilter(t *testing.T) {
	router, client := setupIntegration(t)
	defer client.Close()

	// FIX: Use "/events/" for all POST requests here too
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/events/",
		bytes.NewReader([]byte(`{"eventname": "Cheap Concert", "type": "concert", "price": 50}`))))

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/events/",
		bytes.NewReader([]byte(`{"eventname": "Expensive Concert", "type": "concert", "price": 500}`))))

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/events/",
		bytes.NewReader([]byte(`{"eventname": "Puppet Show", "type": "theater", "price": 40}`))))

	// Query
	req := httptest.NewRequest(http.MethodGet, "/events/?type=concert&max_price=100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp domain.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	events := resp.Data.([]interface{})
	found := false
	for _, e := range events {
		eMap := e.(map[string]interface{})
		name := eMap["eventname"].(string)

		if name == "Cheap Concert" {
			found = true
		}
		if name == "Expensive Concert" {
			t.Error("Found expensive concert but filtering by max_price=100")
		}
		if name == "Puppet Show" {
			t.Error("Found theater show but filtering by type=concert")
		}
	}

	if !found {
		t.Error("Did not find expected 'Cheap Concert'")
	}
}
