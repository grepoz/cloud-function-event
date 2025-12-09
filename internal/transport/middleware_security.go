package transport

import "net/http"

// WithSecurityHeaders adds standard HTTP security headers to the response.
func WithSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Prevent MIME-Sniffing
		// Tells the browser to trust the "Content-Type" header explicitly.
		// Prevents attackers from disguising malicious files (e.g., HTML as images).
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// 2. Prevent Clickjacking
		// Stops your site from being embedded in an iframe on another site.
		w.Header().Set("X-Frame-Options", "DENY")

		// 3. XSS Protection
		// Enables the browser's built-in Cross-Site Scripting filter (mostly for older browsers).
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// 4. Force HTTPS (HSTS) - Production Only
		// Tells the browser to ONLY connect via HTTPS for the next year.
		// NOTE: Only enable this if you are serving over HTTPS!
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// 5. Content Security Policy (CSP)
		// For an API returning JSON, "default-src 'none'" is the safest.
		// It prevents the browser from loading ANY external scripts/styles/images
		// if the API response is accidentally rendered as HTML.
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		// 6. Referrer Policy
		// Controls how much referrer information (the URL the user came from) is sent.
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}
