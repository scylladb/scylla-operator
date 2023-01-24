package gapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type mockServerCall struct {
	code int
	body string
}

type mockServer struct {
	upcomingCalls []mockServerCall
	executedCalls []mockServerCall
	server        *httptest.Server
}

func (m *mockServer) Close() {
	m.server.Close()
}

func gapiTestTools(t *testing.T, code int, body string) *Client {
	return gapiTestToolsFromCalls(t, []mockServerCall{{code, body}})
}

func gapiTestToolsFromCalls(t *testing.T, calls []mockServerCall) *Client {
	t.Helper()

	mock := &mockServer{
		upcomingCalls: calls,
	}

	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := mock.upcomingCalls[0]
		if len(calls) > 1 {
			mock.upcomingCalls = mock.upcomingCalls[1:]
		} else {
			mock.upcomingCalls = nil
		}
		w.WriteHeader(call.code)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, call.body)
		mock.executedCalls = append(mock.executedCalls, call)
	}))

	tr := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(mock.server.URL)
		},
	}

	httpClient := &http.Client{Transport: tr}

	client, err := New("http://my-grafana.com", Config{APIKey: "my-key", Client: httpClient})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		mock.Close()
	})

	return client
}
