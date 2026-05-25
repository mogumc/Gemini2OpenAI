package auth

import (
	"net/http"
	"strings"
)

// Middleware validates the proxy API key.
func Middleware(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if no proxy key is configured
			if apiKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":{"message":"Missing Authorization header","type":"invalid_request_error"}}`, http.StatusUnauthorized)
				return
			}

			// Extract Bearer token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, `{"error":{"message":"Invalid Authorization header format","type":"invalid_request_error"}}`, http.StatusUnauthorized)
				return
			}

			token := parts[1]
			if token != apiKey {
				http.Error(w, `{"error":{"message":"Invalid API key","type":"invalid_request_error"}}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
