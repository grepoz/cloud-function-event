package transport

import (
	"net/http"
	"strings"
)

// WithAuthProtection is a middleware that enforces the "Interim State" logic:
// 1. Authenticated Users (with Token) -> Full Access
// 2. Guest Users (No Token) -> Read Only Access (GET) IF publicRead is true
// 3. Otherwise -> 401 Unauthorized
func WithAuthProtection(next http.Handler, publicRead bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// 1. Check for Authentication Token
		// (In production, use the Firebase Admin SDK to verify the token signature here)
		authHeader := r.Header.Get("Authorization")
		isAuthenticated := strings.HasPrefix(authHeader, "Bearer ") && len(authHeader) > 7

		// IF User is Authenticated: Grant access (Option A behavior)
		if isAuthenticated {
			next.ServeHTTP(w, r)
			return
		}

		// 2. Guest Access Logic (The "For Now" Control)
		// IF Public Read is Enabled AND Request is Read-Only (GET)
		if publicRead && r.Method == http.MethodGet {
			// Optional: Add a header to indicate this response was public
			w.Header().Set("X-Access-Type", "Public-Preview")
			next.ServeHTTP(w, r)
			return
		}

		// 3. Default: Deny Access
		http.Error(w, "Unauthorized: Login required", http.StatusUnauthorized)
	})
}
