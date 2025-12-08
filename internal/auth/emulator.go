package auth

import (
	"encoding/base64"
	"encoding/json"
)

// GenerateEmulatorToken creates an unsigned JWT accepted by the Firebase Auth Emulator.
func GenerateEmulatorToken(projectID, uid string) string {
	if projectID == "" {
		projectID = "local-project-id"
	}

	header := `{"alg":"none","typ":"JWT"}`
	payload := map[string]interface{}{
		"iss":       "https://securetoken.google.com/" + projectID,
		"aud":       projectID,
		"auth_time": 1,
		"user_id":   uid,
		"sub":       uid,
		"iat":       1,
		"exp":       9999999999, // Never expire
	}

	pBytes, _ := json.Marshal(payload)
	enc := base64.RawURLEncoding

	// Format: Header.Payload.Signature (empty for none alg)
	return enc.EncodeToString([]byte(header)) + "." + enc.EncodeToString(pBytes) + "."
}
