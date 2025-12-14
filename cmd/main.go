package main

import (
	"context"
	"log"
	"os"
	"time"

	// 1. Load .env BEFORE importing the function package
	_ "github.com/joho/godotenv/autoload"

	// Blank-import the function package so the init() runs
	_ "bibently/backend"

	emulatorAuth "bibently/backend/internal/auth"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
)

// the main function starts the Functions Framework server - only needed when running locally
func main() {
	// 1. Setup Port
	port := "5000"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	// 2. Setup Hostname (Local Only)
	hostname := ""
	if localOnly := os.Getenv("LOCAL_ONLY"); localOnly == "true" {
		hostname = "127.0.0.1"
	}

	// 3. NEW: Create Local Admin User if Emulator is detected
	// This ensures the admin UID exists in the Auth Emulator so tokens are valid.
	if os.Getenv("FIREBASE_AUTH_EMULATOR_HOST") != "" {
		go createLocalAdminUser()
	}

	log.Println("Server starting on http://127.0.0.1:" + port)
	log.Println("Swagger UI: http://127.0.0.1:" + port + "/swagger/index.html")

	// 4. Start Server
	if err := funcframework.StartHostPort(hostname, port); err != nil {
		log.Fatalf("funcframework.StartHostPort: %v\n", err)
	}
}

func createLocalAdminUser() {
	// Give the server/emulator a split second to settle
	time.Sleep(1 * time.Second)

	ctx := context.Background()
	adminUID := os.Getenv("FIRESTORE_ADMIN_UID")
	if adminUID == "" {
		log.Println("⚠️  Skipping local user creation: FIRESTORE_ADMIN_UID not set")
		return
	}

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = "local-project-id"
	}

	conf := &firebase.Config{ProjectID: projectID}
	app, err := firebase.NewApp(ctx, conf)
	if err != nil {
		log.Printf("⚠️  [Admin Setup] Failed to init firebase app: %v", err)
		return
	}

	client, err := app.Auth(ctx)
	if err != nil {
		log.Printf("⚠️  [Admin Setup] Failed to get auth client: %v", err)
		return
	}

	// Attempt to create/get user
	u, err := client.GetUser(ctx, adminUID)
	if err == nil {
		log.Printf("✅ [Admin Setup] User '%s' already exists (UID: %s)", u.DisplayName, adminUID)
		return
	}

	params := (&auth.UserToCreate{}).
		UID(adminUID).
		Email("admin@localhost.com").
		EmailVerified(true).
		Password("admin123").
		DisplayName("Local Admin")

	if _, err := client.CreateUser(ctx, params); err != nil {
		log.Printf("❌ [Admin Setup] Failed to create user (Emulator might be down): %v", err)
	} else {
		log.Printf("✅ [Admin Setup] Created user: %s", adminUID)
	}

	// --- B. Generate & Print Token ---
	// Call the refactored function from internal/auth
	token := emulatorAuth.GenerateEmulatorToken(projectID, adminUID)

	log.Println("---------------------------------------------------------")
	log.Printf("🔑 ADMIN TOKEN (Copy to Swagger 'Authorize'):\nBearer %s", token)
	log.Println("---------------------------------------------------------")
}
