// Copyright (C) 2017 ScyllaDB

package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
)

// ValidateAuthToken is http server middleware that checks if Authorization
// header contains `Bearer token`.
// If not the execution would be held for the penalty duration and then 401
// status code would be returned.
// If token is empty it immediately returns the next handler.
func ValidateAuthToken(next http.Handler, token string, penalty time.Duration) http.Handler {
	if token == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !secureCompare(bearerAuth(r), token) {
			if penalty > 0 {
				time.Sleep(penalty)
			}
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

// bearerAuth returns the token provided in the request's Authorization header.
func bearerAuth(r *http.Request) (token string) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return
	}
	return parseBearerAuth(auth)
}

func parseBearerAuth(auth string) (token string) {
	const prefix = "Bearer "
	// Case insensitive prefix match. See Issue 22736.
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return ""
	}
	return auth[len(prefix):]
}

func secureCompare(x, y string) bool {
	return subtle.ConstantTimeCompare([]byte(x), []byte(y)) == 1
}
