package transport

import (
	"context"
	"net/http"
	"os"
	"strings"

	"firebase.google.com/go/v4/auth"
)

// 1. Define a custom unexported type to prevent collisions
type contextKey string

// 2. Define the key constant. We export it so other packages in your app can read it.
const UserContextKey contextKey = "user"

func WithAuthProtection(next http.Handler, authClient *auth.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		authHeader := r.Header.Get("Authorization")
		var token *auth.Token
		var err error

		// 1. Verify Token FIRST
		if strings.HasPrefix(authHeader, "Bearer ") {
			idToken := strings.TrimPrefix(authHeader, "Bearer ")
			token, err = authClient.VerifyIDToken(r.Context(), idToken)
		}

		// Check if user is fully authenticated
		isAuthenticated := token != nil && err == nil

		// 2. NOW check for Admin Role (Fix applied here)
		// If it's a write operation (POST/PUT/DELETE), ensure the user is the Admin
		adminUID := os.Getenv("FIRESTORE_ADMIN_UID")
		if r.Method != http.MethodGet {
			if !isAuthenticated || token.UID != adminUID {
				http.Error(w, "Forbidden: Admins only", http.StatusForbidden)
				return
			}
		}

		if isAuthenticated {
			// Inject user info into context
			ctx := context.WithValue(r.Context(), UserContextKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// 3. Guest Access Logic (Restricted to /events only)
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/events") {
			w.Header().Set("X-Access-Type", "Public-Preview")
			next.ServeHTTP(w, r)
			return
		}

		// 4. Deny Access
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, err = w.Write([]byte(`{"error": "Unauthorized: Valid Bearer token required"}`))
		if err != nil {
			return
		}
	})
}
