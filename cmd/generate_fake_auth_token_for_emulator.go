package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	// 1. Define your desired Admin UID
	uid := os.Getenv("FIRESTORE_ADMIN_UID")
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")

	// 2. Create a fake token (logic from your rules_test.go)
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
	token := enc.EncodeToString([]byte(header)) + "." + enc.EncodeToString(pBytes) + "."

	fmt.Printf("Bearer %s\n", token)
}
