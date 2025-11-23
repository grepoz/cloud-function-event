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

// setupIntegration prepares the test infrastructure connected to the emulator
func setupIntegration(t *testing.T) (*transport.Router, *firestore.Client) {

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

	// 1. Firestore Client
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to create firestore client: %v", err)
	}

	// 2. Build Layers (Updated for multiple services)
	repos := repository.NewFirestoreRepository(client)

	eventSvc := service.NewEventService(repos.Events)
	trackingSvc := service.NewTrackingService(repos.Tracking)

	// 3. Use the Router instead of the single Handler
	router := transport.NewRouter(eventSvc, trackingSvc)

	return router, client
}

// TestIntegration_Tracking verifies the new /tracking endpoint [NEW]
func TestIntegration_Tracking(t *testing.T) {
	router, client := setupIntegration(t)
	defer client.Close()

	// 1. Track an event (POST /tracking)
	trackPayload := map[string]string{
		"action":  "button_click",
		"payload": "signup_page",
	}
	body, _ := json.Marshal(trackPayload)

	// Note the path /tracking
	req := httptest.NewRequest(http.MethodPost, "/tracking", bytes.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected 201 Created, got %d", w.Code)
	}

	// 2. Retrieve tracking logs (GET /tracking)
	reqGet := httptest.NewRequest(http.MethodGet, "/tracking", nil)
	wGet := httptest.NewRecorder()

	router.ServeHTTP(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", wGet.Code)
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(wGet.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify we found our tracked action
	dataBytes, _ := json.Marshal(resp.Data)
	if !bytes.Contains(dataBytes, []byte("button_click")) {
		t.Errorf("Response did not contain tracked action. Got: %s", string(dataBytes))
	}
}

func TestIntegration_CreateAndGetEvent(t *testing.T) {
	router, client := setupIntegration(t)
	defer client.Close()

	// Uses /events prefix now?
	// NOTE: In the Router implementation provided previously, we strip prefix.
	// If we send request to "/events", the router handles it.

	newEvent := map[string]interface{}{
		"eventname":      "Integration Concert",
		"city":           "Warsaw",
		"price":          150.00,
		"organizer_name": "Test Org",
		"type":           "concert",
	}
	body, _ := json.Marshal(newEvent)

	// Important: Request URL must match the Router logic in transport/handler.go
	// If Router checks `strings.HasPrefix(path, "/events")`, we must use /events/
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	eventID, ok := resp.Data.(string)
	if !ok || eventID == "" {
		t.Fatalf("Expected event ID in response, got %v", resp.Data)
	}
	t.Logf("Created event with ID: %s", eventID)

	// --- KROK 2: Pobieranie Eventu (GET) ---
	// Symulujemy zapytanie GET ?id=...
	reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/events?id=%s", eventID), nil)
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
	handler, client := setupIntegration(t)
	defer func(client *firestore.Client) {
		err := client.Close()
		if err != nil {
			t.Fatalf("Failed to close firestore client: %v", err)
		}
	}(client)

	// Tworzymy event, żeby lista nie była pusta
	createReq := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader([]byte(`{"eventname":"List Me", "city":"Cracow", "price": 50}`)))
	handler.ServeHTTP(httptest.NewRecorder(), createReq)

	// --- KROK 1: Listowanie (GET z filtrami) ---
	req := httptest.NewRequest(http.MethodGet, "/events/?city=Cracow", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Sprawdzamy czy 'data' jest listą (slice)
	dataSlice, ok := resp.Data.([]interface{})
	if !ok {
		// Jeśli JSON decode zmapował to inaczej, sprawdźmy surowy string
		t.Logf("Data type is %T", resp.Data)
	} else {
		if len(dataSlice) == 0 {
			t.Error("Expected non-empty list of events")
		}
	}
}

func TestIntegration_UpdateAndDelete(t *testing.T) {
	handler, client := setupIntegration(t)
	defer func(client *firestore.Client) {
		err := client.Close()
		if err != nil {
			t.Fatalf("Failed to close firestore client: %v", err)
		}
	}(client)

	// 1. Tworzymy wydarzenie do edycji
	createBody := `{"eventname": "To Change", "city": "Old City", "price": 100}`
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader([]byte(createBody))))

	var resp domain.APIResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		return
	}
	eventID := resp.Data.(string)

	// 2. Aktualizacja (PUT) - Zmieniamy miasto i cenę
	updateBody := `{"city": "New City", "price": 200.50}`
	reqUpdate := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/events?id=%s", eventID), bytes.NewReader([]byte(updateBody)))
	wUpdate := httptest.NewRecorder()

	handler.ServeHTTP(wUpdate, reqUpdate)

	if wUpdate.Code != http.StatusOK {
		t.Fatalf("Update failed, got %d", wUpdate.Code)
	}

	// 3. Weryfikacja zmian (GET)
	wGet := httptest.NewRecorder()
	handler.ServeHTTP(wGet, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/events?id=%s", eventID), nil))

	var eventResp domain.APIResponse
	err = json.NewDecoder(wGet.Body).Decode(&eventResp)
	if err != nil {
		return
	}

	// Konwersja mapy na JSON, by łatwo sprawdzić wartości (unikanie problemów z rzutowaniem map[string]interface{})
	eventBytes, _ := json.Marshal(eventResp.Data)
	eventStr := string(eventBytes)

	if !bytes.Contains(eventBytes, []byte("New City")) {
		t.Errorf("Event not updated. Got: %s", eventStr)
	}
	// Sprawdzenie czy cena się zmieniła (float w JSON może być różnie formatowany, więc szukamy stringa lub parsujemy)
	if !bytes.Contains(eventBytes, []byte("200.5")) {
		t.Errorf("Price not updated. Got: %s", eventStr)
	}

	// 4. Usuwanie (DELETE)
	wDel := httptest.NewRecorder()
	handler.ServeHTTP(wDel, httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/events?id=%s", eventID), nil))

	if wDel.Code != http.StatusOK {
		t.Errorf("Delete failed, got %d", wDel.Code)
	}

	// 5. Weryfikacja usunięcia (GET -> 404)
	wGetDeleted := httptest.NewRecorder()
	handler.ServeHTTP(wGetDeleted, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/events?id=%s", eventID), nil))

	if wGetDeleted.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found after delete, got %d", wGetDeleted.Code)
	}
}

func TestIntegration_Pagination(t *testing.T) {
	handler, client := setupIntegration(t)
	defer func(client *firestore.Client) {
		err := client.Close()
		if err != nil {
			t.Fatalf("Failed to close firestore client: %v", err)
		}
	}(client)

	// 1. Przygotowanie danych: Tworzymy 4 wydarzenia
	// Używamy unikalnych nazw, by łatwo je rozpoznać
	titles := []string{"Page1_A", "Page1_B", "Page2_A", "Page2_B"}
	for _, title := range titles {
		body := fmt.Sprintf(`{"eventname": "%s", "city": "PaginationTest", "start_time": "%s"}`,
			title, time.Now().Add(time.Hour).Format(time.RFC3339))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader([]byte(body))))
	}

	// 2. Pobieramy pierwszą stronę (PageSize = 2)
	// Sortowanie po created_at lub start_time jest domyślne. Filtrujemy po unikalnym mieście dla testu.
	reqPage1 := httptest.NewRequest(http.MethodGet, "/events?city=PaginationTest&page_size=2", nil)
	wPage1 := httptest.NewRecorder()
	handler.ServeHTTP(wPage1, reqPage1)

	var resp1 domain.APIResponse
	err := json.NewDecoder(wPage1.Body).Decode(&resp1)
	if err != nil {
		return
	}

	data1 := resp1.Data.([]interface{})
	if len(data1) != 2 {
		t.Fatalf("Expected 2 events on page 1, got %d", len(data1))
	}

	// Sprawdzamy czy mamy token do następnej strony
	if resp1.Meta == nil || resp1.Meta.NextPageToken == "" {
		t.Fatal("Expected NextPageToken in response")
	}
	token := resp1.Meta.NextPageToken

	// 3. Pobieramy drugą stronę używając tokenu
	reqPage2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/events?city=PaginationTest&page_size=2&page_token=%s", token), nil)
	wPage2 := httptest.NewRecorder()
	handler.ServeHTTP(wPage2, reqPage2)

	var resp2 domain.APIResponse
	err = json.NewDecoder(wPage2.Body).Decode(&resp2)
	if err != nil {
		return
	}

	data2 := resp2.Data.([]interface{})
	if len(data2) != 2 {
		t.Fatalf("Expected 2 events on page 2, got %d", len(data2))
	}

	// Sprawdzamy czy nie pobraliśmy tych samych danych (proste sprawdzenie po JSON)
	json1, _ := json.Marshal(data1)
	json2, _ := json.Marshal(data2)
	if string(json1) == string(json2) {
		t.Error("Page 1 and Page 2 contain identical data (pagination failed?)")
	}
}

func TestIntegration_ComplexFilter(t *testing.T) {
	handler, client := setupIntegration(t)
	defer func(client *firestore.Client) {
		err := client.Close()
		if err != nil {
			t.Fatalf("Failed to close firestore client: %v", err)
		}
	}(client)

	// Scenariusz: Szukamy tanich koncertów
	// Event A: Tani (50), pasuje
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/events",
		bytes.NewReader([]byte(`{"eventname": "Cheap Concert", "type": "concert", "price": 50}`))))

	// Event B: Drogi (500), nie pasuje ceną
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/events",
		bytes.NewReader([]byte(`{"eventname": "Expensive Concert", "type": "concert", "price": 500}`))))

	// Event C: Tani (40), ale inny typ ("theater"), nie pasuje typem
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/events",
		bytes.NewReader([]byte(`{"eventname": "Puppet Show", "type": "theater", "price": 40}`))))

	// Zapytanie: type=concert AND max_price=100
	req := httptest.NewRequest(http.MethodGet, "/events?type=concert&max_price=100", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var resp domain.APIResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		return
	}

	events := resp.Data.([]interface{})

	// Oczekujemy tylko 1 wyniku ("Cheap Concert")
	// Uwaga: Jeśli w bazie emulatora zostały śmieci z innych testów, filtr powinien je odrzucić,
	// chyba że przypadkiem pasują. Dla pewności w testach integracyjnych czasem dodaje się unikalny suffix do zapytań.

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
