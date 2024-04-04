// Copyright (C) 2017 ScyllaDB

package auth

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/scylladb/scylla-manager/v3/pkg/util/httpx"
)

// AddToken sets authorization header. If token is empty it immediately returns
// the next handler.
func AddToken(next http.RoundTripper, token string) http.RoundTripper {
	if token == "" {
		return next
	}

	return httpx.RoundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
		r := httpx.CloneRequest(req)
		r.Header.Set("Authorization", "Bearer "+token)
		return next.RoundTrip(r)
	})
}

// ValidateToken is http server middleware that checks if Authorization header
// contains `Bearer token`.
// If not the execution would be held for the penalty duration and then 401
// status code with provided body would be returned.
// If token is empty it immediately returns the next handler.
func ValidateToken(token string, penalty time.Duration,
	unauthorizedBody json.RawMessage,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if token == "" {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !secureCompare(bearerAuth(r), token) {
				if penalty > 0 {
					time.Sleep(penalty)
				}
				w.WriteHeader(http.StatusUnauthorized)
				w.Write(unauthorizedBody) // nolint: errcheck
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
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
