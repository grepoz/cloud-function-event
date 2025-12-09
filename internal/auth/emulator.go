package auth

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

// GenerateEmulatorToken creates an unsigned JWT accepted by the Firebase Auth Emulator.
func GenerateEmulatorToken(projectID, uid string) string {
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
		"email":     "admin@localhost.com",
	}

	pBytes, _ := json.Marshal(payload)
	enc := base64.RawURLEncoding

	// Format: Header.Payload.Signature (empty for none alg)
	return enc.EncodeToString([]byte(header)) + "." + enc.EncodeToString(pBytes) + "."
}
