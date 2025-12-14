package integration_tests

import (
	"bibently/backend/internal/domain"
	"bibently/backend/internal/repository"
	"bibently/backend/internal/service"
	"bibently/backend/internal/transport"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

func setupIntegration(t *testing.T) (http.Handler, *firestore.Client) {

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
	withFirestore(t, func(t *testing.T, router http.Handler, client *firestore.Client) {

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
	})
}

func TestIntegration_CreateAndGetEvent(t *testing.T) {
	withFirestore(t, func(t *testing.T, router http.Handler, client *firestore.Client) {

		newEvent := map[string]interface{}{
			"event_name": "Integration Concert",
			"city":       "Warsaw",
			"type":       "concert",
			"start_time": time.Now().Add(2 * time.Hour).Format(time.RFC3339),
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
	})
}

func TestIntegration_ListEvents(t *testing.T) {
	withFirestore(t, func(t *testing.T, router http.Handler, client *firestore.Client) {

		// 1. Setup: Create an event so the list is not empty
		// IMPORTANT: Use trailing slash "/events/" for POST
		createBody := `{"event_name":"List Me", "city":"Cracow", "price": 50, "start_time":"2024-12-31T20:00:00Z", "type":"theater"}`
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
			if len(dataSlice) != 1 {
				t.Errorf("Got %d events, expected 1", len(dataSlice))
			}
		}
	})
}

func TestIntegration_UpdateAndDelete(t *testing.T) {
	withFirestore(t, func(t *testing.T, router http.Handler, client *firestore.Client) {

		// Create
		createBody := `{"event_name":"To Change", "city":"Cracow", "price": 50, "start_time":"2024-12-31T20:00:00Z", "type":"theater"}`
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
	})
}

func TestIntegration_Pagination(t *testing.T) {
	withFirestore(t, func(t *testing.T, router http.Handler, client *firestore.Client) {

		// 1. Prepare data
		titles := []string{"Page1_A", "Page1_B", "Page2_A", "Page2_B"}
		for _, title := range titles {
			body := fmt.Sprintf(`{"event_name": "%s", "city": "PaginationTest", "type": "concert", "start_time": "%s"}`,
				title, time.Now().Add(time.Hour).Format(time.RFC3339))

			req := httptest.NewRequest(http.MethodPost, "/events/", bytes.NewReader([]byte(body)))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Fatalf("Failed to setup test data. Expected 201, got %d. Body: %s", w.Code, w.Body.String())
			}
		}

		// 2. Get Page 1
		reqPage1 := httptest.NewRequest(http.MethodGet, "/events/?city=PaginationTest&page_size=2&sort_key=event_name&sort_dir=asc", nil)
		wPage1 := httptest.NewRecorder()
		router.ServeHTTP(wPage1, reqPage1)

		var resp1 domain.APIPaginationResponse
		if err := json.NewDecoder(wPage1.Body).Decode(&resp1); err != nil {
			t.Fatal(err)
		}

		data1 := resp1.Data.([]interface{})
		if len(data1) != 2 {
			t.Fatalf("Expected 2 events on page 1, got %d", len(data1))
		}

		// Check for Next Page Token
		if resp1.Meta == nil || resp1.Meta.NextPageToken == "" {
			t.Fatal("Expected NextPageToken in response metadata")
		}
		token := resp1.Meta.NextPageToken

		// 3. Get Page 2 using Token
		reqPage2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/events/?city=PaginationTest&page_size=2&sort_key=event_name&sort_dir=asc&page_token=%s", token), nil)
		wPage2 := httptest.NewRecorder()
		router.ServeHTTP(wPage2, reqPage2)

		var resp2 domain.APIPaginationResponse
		if err := json.NewDecoder(wPage2.Body).Decode(&resp2); err != nil {
			t.Fatal(err)
		}

		data2 := resp2.Data.([]interface{})
		if len(data2) != 2 {
			t.Fatalf("Expected 2 events on page 2, got %d", len(data2))
		}

		// Verify different data (simple check)
		title1 := data1[0].(map[string]interface{})["EventName"]
		title2 := data2[0].(map[string]interface{})["EventName"]
		if title1 == title2 {
			t.Error("Page 1 and Page 2 data appear identical")
		}
	})
}

func TestIntegration_ComplexFilter(t *testing.T) {
	withFirestore(t, func(t *testing.T, router http.Handler, client *firestore.Client) {

		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/events/",
			bytes.NewReader([]byte(`{"event_name": "Cheap Concert", "city": "Gdansk", "type": "concert", "price": 50, "start_time":"2024-12-31T20:00:00Z"}`))))

		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/events/",
			bytes.NewReader([]byte(`{"event_name": "Expensive Concert", "city": "Gdansk", "type": "concert", "price": 500, "start_time":"2024-12-31T20:00:00Z"}`))))

		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/events/",
			bytes.NewReader([]byte(`{"event_name": "Puppet Show", "city": "Gdansk", "type": "theater", "price": 40, "start_time":"2024-12-31T20:00:00Z"}`))))

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
			name := eMap["EventName"].(string)

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
	})
}

func TestIntegration_BatchCreateEvents(t *testing.T) {
	withFirestore(t, func(t *testing.T, router http.Handler, client *firestore.Client) {

		// 1. Prepare Batch JSON
		bodyJSON := `{
			"events": [
				{
					"event_name": "Batch Event 1",
					"city": "Gdansk",
					"type": "concert",
					"start_time": "2025-05-01T20:00:00Z"
				},
				{
					"event_name": "Batch Event 2",
					"city": "Gdansk",
					"type": "conference",
					"start_time": "2025-06-01T09:00:00Z"
				}
			]
		}`

		// 2. Execute Request
		req := httptest.NewRequest(http.MethodPost, "/events/batch", bytes.NewReader([]byte(bodyJSON)))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 3. Verify Response
		if w.Code != http.StatusCreated {
			t.Fatalf("Expected 201 Created, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp domain.APIResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if resp.Data != "Successfully created 2 events" {
			t.Errorf("Unexpected response message: %v", resp.Data)
		}

		// 4. Verify Persistence (Check count)
		iter := client.Collection("events").Documents(context.Background())
		docs, err := iter.GetAll()
		if err != nil {
			t.Fatal(err)
		}
		if len(docs) != 2 {
			t.Errorf("Expected 2 documents in Firestore, found %d", len(docs))
		}
	})
}

func withFirestore(t *testing.T, testFunc func(t *testing.T, router http.Handler, client *firestore.Client)) {
	t.Helper()

	router, client := setupIntegration(t)

	t.Cleanup(func() {
		cleanupFirestore(t, client)
		err := client.Close()
		if err != nil {
			return
		}
	})

	testFunc(t, router, client)
}

func cleanupFirestore(t *testing.T, client *firestore.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collections := []string{"events", "tracking"}

	for _, colName := range collections {
		iter := client.Collection(colName).Documents(ctx)
		defer iter.Stop()

		batch := client.Batch()
		count := 0

		for {
			doc, err := iter.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				t.Fatalf("Failed to iterate documents in %s: %v", colName, err)
			}

			batch.Delete(doc.Ref)
			count++

			if count == 500 {
				if _, err := batch.Commit(ctx); err != nil {
					t.Fatalf("Batch commit failed: %v", err)
				}
				batch = client.Batch()
				count = 0
			}
		}

		if count > 0 {
			if _, err := batch.Commit(ctx); err != nil {
				t.Fatalf("Final batch commit failed: %v", err)
			}
		}
	}
}
