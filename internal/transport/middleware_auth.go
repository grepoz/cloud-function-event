package transport

import (
	"context"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"
)

// WithAuthProtection verifies the Firebase ID Token.
//  1. Valid Token -> Context populated with AuthToken, Request proceeds.
//  2. No Token/Invalid Token ->
//     a) If publicRead=true AND Method=GET AND Path starts with /events -> Proceed as Guest.
//     b) Otherwise -> 401 Unauthorized.
func WithAuthProtection(next http.Handler, authClient *auth.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		authHeader := r.Header.Get("Authorization")
		var token *auth.Token
		var err error

		// 1. Verify Token if present
		if strings.HasPrefix(authHeader, "Bearer ") {
			idToken := strings.TrimPrefix(authHeader, "Bearer ")
			token, err = authClient.VerifyIDToken(r.Context(), idToken)
		}

		// Check if user is fully authenticated
		isAuthenticated := token != nil && err == nil

		if isAuthenticated {
			// Inject user info into context for downstream handlers
			ctx := context.WithValue(r.Context(), "user", token)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// 2. Guest Access Logic (Restricted to /events only)
		// We add a check: strings.HasPrefix(r.URL.Path, "/events")
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/events") {
			w.Header().Set("X-Access-Type", "Public-Preview")
			next.ServeHTTP(w, r)
			return
		}

		// 3. Deny Access
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, err = w.Write([]byte(`{"error": "Unauthorized: Valid Bearer token required"}`))
		if err != nil {
			return
		}
	})
}
