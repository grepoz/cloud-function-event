package main

import (
	"fmt"
	"os"

	"bibently.com/backend/internal/auth"
)

func main() {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = "local-project-id"
	}
	uid := os.Getenv("FIRESTORE_ADMIN_UID")
	if uid == "" {
		uid = "admin_user_xyz_123_secret_id"
	}

	token := auth.GenerateEmulatorToken(projectID, uid)
	fmt.Printf("Bearer %s\n", token)
}
