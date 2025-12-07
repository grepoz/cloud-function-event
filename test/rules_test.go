package test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
)

// TestFirestoreSecurityRules verifies that firestore.rules are enforced correctly.
// It bypasses the Go Admin Client (which ignores rules) and hits the Emulator REST API directly.
func TestFirestoreSecurityRules(t *testing.T) {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	adminUID := os.Getenv("FIRESTORE_ADMIN_UID")
	emulatorHost := os.Getenv("FIRESTORE_EMULATOR_HOST")

	if emulatorHost == "" {
		t.Skip("Skipping security rules test: FIRESTORE_EMULATOR_HOST not set")
	}
	if adminUID == "" {
		t.Fatal("FIRESTORE_ADMIN_UID must be set to run rule tests")
	}

	// 1. Test Admin User (Should Succeed)
	t.Run("AdminUser_CanWrite", func(t *testing.T) {
		token := createEmulatorToken(adminUID, projectID)
		if err := tryWriteEvent(emulatorHost, projectID, token); err != nil {
			t.Errorf("Admin user failed to write to DB: %v", err)
		}
	})

	// 2. Test Stranger User (Should Fail)
	t.Run("OtherUser_CannotWrite", func(t *testing.T) {
		token := createEmulatorToken("stranger_user_123", projectID)
		err := tryWriteEvent(emulatorHost, projectID, token)
		if err == nil {
			t.Error("Security breach: Non-admin user was able to write to DB!")
		} else {
			t.Logf("Success: Blocked unauthorized access as expected. Error: %v", err)
		}
	})
}

// createEmulatorToken generates an unsigned JWT that the Firestore Emulator accepts
// This allows us to "create" a user session on the fly for testing.
func createEmulatorToken(uid string, projectID string) string {
	header := `{"alg":"none","typ":"JWT"}`
	payload := map[string]interface{}{
		"iss":       "https://securetoken.google.com/" + projectID,
		"aud":       projectID,
		"auth_time": 1,
		"user_id":   uid,
		"sub":       uid,
		"iat":       1,
		"exp":       9999999999,
	}

	pBytes, _ := json.Marshal(payload)
	enc := base64.RawURLEncoding
	return enc.EncodeToString([]byte(header)) + "." + enc.EncodeToString(pBytes) + "."
}

// tryWriteEvent attempts to write a document via the Firestore REST API
func tryWriteEvent(host, project, token string) error {
	// Construct Emulator REST URL
	url := fmt.Sprintf("http://%s/v1/projects/%s/databases/(default)/documents/events", host, project)

	// Minimal valid Firestore Document JSON
	body := `{"fields": {
		"event_name": {"stringValue": "Security Rule Test"},
		"city": {"stringValue": "TestCity"},
		"type": {"stringValue": "concert"},
		"price": {"doubleValue": 10},
		"start_time": {"timestampValue": "2025-01-01T10:00:00Z"}
	}}`

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("write failed with status: %d", resp.StatusCode)
	}
	return nil
}
