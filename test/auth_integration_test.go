package test

import (
	"bytes"
	"cloud-function-event/internal/repository"
	"cloud-function-event/internal/service"
	"cloud-function-event/internal/transport"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

const TestAdminUID = "admin_user"
const TestProjectID = "local-project-id"

// generateValidEmulatorToken creates a fresh, unsigned JWT for the emulator
func generateValidEmulatorToken(uid, projectID string) string {
	header := `{"alg":"none","typ":"JWT"}`
	now := time.Now().Unix()

	payload := map[string]interface{}{
		"iss":       "https://securetoken.google.com/" + projectID,
		"aud":       projectID,
		"auth_time": now,
		"user_id":   uid,
		"sub":       uid,
		"iat":       now,
		"exp":       now + 3600, // Expires in 1 hour
		"email":     "admin@test.local",
	}

	pBytes, _ := json.Marshal(payload)
	enc := base64.RawURLEncoding
	// Important: The trailing dot indicates an empty signature
	return enc.EncodeToString([]byte(header)) + "." + enc.EncodeToString(pBytes) + "."
}

// ensureUserExists creates the user in the Auth Emulator if missing
func ensureUserExists(ctx context.Context, client *auth.Client, uid string) error {
	_, err := client.GetUser(ctx, uid)
	if err == nil {
		return nil // User exists
	}

	params := (&auth.UserToCreate{}).
		UID(uid).
		Email("admin@test.local").
		EmailVerified(true).
		Password("password123").
		DisplayName("Integration Test Admin")

	_, err = client.CreateUser(ctx, params)
	return err
}

func setupAuthIntegration(t *testing.T) (http.Handler, *firestore.Client) {
	t.Helper()

	// 1. Force Emulator Hosts
	// Must be set BEFORE firebase.NewApp is called
	_ = os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080")
	_ = os.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "localhost:9099")

	ctx := context.Background()

	// 2. Initialize Firestore
	client, err := firestore.NewClient(ctx, TestProjectID)
	if err != nil {
		t.Fatalf("Failed to create firestore client: %v", err)
	}

	// 3. Initialize Firebase App & Auth
	conf := &firebase.Config{ProjectID: TestProjectID}
	app, err := firebase.NewApp(ctx, conf)
	if err != nil {
		t.Fatalf("Failed to init firebase app: %v", err)
	}
	authClient, err := app.Auth(ctx)
	if err != nil {
		t.Fatalf("Failed to get auth client: %v", err)
	}

	// 4. Create User in Emulator
	if err := ensureUserExists(ctx, authClient, TestAdminUID); err != nil {
		t.Fatalf("Failed to ensure admin user exists: %v", err)
	}

	// 5. Build Application Stack
	eventRepo := repository.NewEventRepository(client)
	trackingRepo := repository.NewTrackingRepository(client)
	eventSvc := service.NewEventService(eventRepo)
	trackingSvc := service.NewTrackingService(trackingRepo)

	router := transport.NewRouter(eventSvc, trackingSvc)
	protectedHandler := transport.WithAuthProtection(router, authClient)

	return protectedHandler, client
}

func TestAuth_Strict_Blocking(t *testing.T) {
	handler, client := setupAuthIntegration(t)
	t.Cleanup(func() { client.Close() })

	t.Run("Allow_Unauthenticated_Read", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/events/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("Block_Unauthenticated_Write", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"event_name": "Hack"}`))
		req := httptest.NewRequest(http.MethodPost, "/events/", body)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized, got %d", w.Code)
		}
	})
}

// TestAuth_Authenticated_Access: Valid Token Logic
func TestAuth_Authenticated_Access(t *testing.T) {
	handler, client := setupAuthIntegration(t)
	t.Cleanup(func() { client.Close() })
	// Ensure we don't crash on cleanup by closing client last (LIFO)
	// You may need to copy 'cleanupFirestore' from integration_test.go or remove this line if it causes issues
	// t.Cleanup(func() { cleanupFirestore(t, client) })

	// Generate fresh token
	token := generateValidEmulatorToken(TestAdminUID, TestProjectID)
	authHeader := "Bearer " + token

	t.Run("Allow_Authenticated_Write", func(t *testing.T) {
		// Valid Event Payload
		bodyStr := `{"event_name": "Auth Test Event", "city": "Warsaw", "type": "concert", "price": 100, "start_time": "2024-12-31T20:00:00Z"}`
		req := httptest.NewRequest(http.MethodPost, "/events/", bytes.NewReader([]byte(bodyStr)))
		req.Header.Set("Authorization", authHeader)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201 Created, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Allow_Authenticated_Read", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/events/", nil)
		req.Header.Set("Authorization", authHeader)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

func TestAuth_Guest_Mode(t *testing.T) {
	handler, client := setupAuthIntegration(t)
	t.Cleanup(func() { client.Close() })

	t.Run("Allow_Guest_Read", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/events/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for guest read, got %d", w.Code)
		}
		if w.Header().Get("X-Access-Type") != "Public-Preview" {
			t.Errorf("Expected X-Access-Type header, got empty")
		}
	})

	t.Run("Block_Guest_Write", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"event_name": "Hack"}`))
		req := httptest.NewRequest(http.MethodPost, "/events/", body)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized for guest write, got %d", w.Code)
		}
	})
}

func TestAuth_Guest_Mode_Restricted(t *testing.T) {
	// Setup with publicRead = true
	handler, client := setupAuthIntegration(t)
	defer client.Close() // Ensure you use your robust cleanup logic here

	// 1. Events should be ALLOWED
	t.Run("Allow_Guest_Events", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/events/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for guest events, got %d", w.Code)
		}
	})

	// 2. Tracking should be BLOCKED (even if publicRead is true)
	t.Run("Block_Guest_Tracking", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tracking/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized for guest tracking, got %d", w.Code)
		}
	})
}
