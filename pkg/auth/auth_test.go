// Copyright (C) 2017 ScyllaDB

package auth

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scylladb/scylla-operator/pkg/util/timeutc"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/util/httpx"
)

func TestParseBearerAuth(t *testing.T) {
	t.Parallel()

	table := []struct {
		Name   string
		Header string
		Token  string
	}{
		{
			Name:   "empty header",
			Header: "",
			Token:  "",
		},
		{
			Name:   "bearer canonical token",
			Header: "Bearer token",
			Token:  "token",
		},
		{
			Name:   "bearer case mismatch token",
			Header: "bEaReR token",
			Token:  "token",
		},
		{
			Name:   "basic auth",
			Header: "Basic foobar",
			Token:  "",
		},
	}

	for i := range table {
		test := table[i]

		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			if token := parseBearerAuth(test.Header); token != test.Token {
				t.Error("expected", test.Token, "got", token)
			}
		})
	}
}

func TestValidateTokenNoToken(t *testing.T) {
	t.Parallel()

	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	r := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	w := httptest.NewRecorder()

	ValidateToken("", 0, nil)(h).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Error("expected status 200 got", w)
	}
}

func TestValidateTokenSuccess(t *testing.T) {
	t.Parallel()

	const token = "token"
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	r := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	ValidateToken(token, 0, nil)(h).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Error("expected status 200 got", w)
	}
}

func TestValidateTokenFailure(t *testing.T) {
	t.Parallel()

	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("this must not be called")
	})

	verify := func(t *testing.T, r *http.Request, penalty time.Duration) {
		t.Helper()

		var bodyError = json.RawMessage(`{"message":"unauthorized","code":401}`)

		w := httptest.NewRecorder()
		ValidateToken("token", penalty, bodyError)(h).ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Error("expected status 401 got", w)
		}
		responseBody, err := ioutil.ReadAll(ioutil.NopCloser(w.Result().Body))
		if err != nil {
			t.Error("expected nil err, got", err)
		}
		if cmp.Diff(string(responseBody), string(bodyError)) != "" {
			t.Error("wrong response body, got", string(responseBody))
		}
	}

	t.Run("no token", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		verify(t, r, 0)
	})

	t.Run("invalid token", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		r.Header.Set("Authorization", "Bearer foobar")
		verify(t, r, 0)
	})

	t.Run("penalty", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		penalty := 150 * time.Millisecond
		start := timeutc.Now()
		verify(t, r, penalty)
		if timeutc.Since(start) < penalty {
			t.Fatal("expected penalty")
		}
	})
}

func TestCrossCheckAddValidateToken(t *testing.T) {
	t.Parallel()

	const token = "token"

	var h http.Handler
	h = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	h = ValidateToken(token, 0, nil)(h)

	var rt http.RoundTripper
	rt = httpx.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Result(), nil
	})
	rt = AddToken(rt, token)

	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	resp, _ := rt.RoundTrip(req)
	if resp.StatusCode != http.StatusOK {
		t.Error("expected status 200 got", resp.StatusCode)
	}
}
