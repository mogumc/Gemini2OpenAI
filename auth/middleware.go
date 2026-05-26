package auth

import (
	"crypto/subtle"
	"encoding/json"
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
				writeAuthError(w, "Missing Authorization header")
				return
			}

			// Extract Bearer token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				writeAuthError(w, "Invalid Authorization header format")
				return
			}

			token := parts[1]
			if subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) != 1 {
				writeAuthError(w, "Invalid API key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// writeAuthError sends a properly JSON-encoded 401 error.
func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"message": msg,
			"type":    "invalid_request_error",
		},
	})
}
