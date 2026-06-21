package stdhttpbench

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type customerCreateRequest struct {
	Name string `json:"name"`
}

type customerCreateResponse struct {
	Message string `json:"message"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func newStdHTTPBenchmarkRequest() *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/customers/abc?verbose=true", strings.NewReader(`{"name":"Alice"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "secret")
	req.Header.Set("X-Trace-Id", "trace-1")
	return req
}

func BenchmarkStdHTTPHandler(b *testing.B) {
	h := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		segments := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
		if len(segments) != 2 || segments[0] != "customers" {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		customerID := segments[1]
		authToken := req.Header.Get("X-Auth-Token")
		if strings.TrimSpace(authToken) == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "missing X-Auth-Token header"})
			return
		}
		var body customerCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "name is required"})
			return
		}
		w.Header().Set("X-Request-Id", "req-"+customerID)
		w.Header().Set("X-Trace-Id", req.Header.Get("X-Trace-Id"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(customerCreateResponse{Message: fmt.Sprintf("customer %s created", customerID)})
	})

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := newStdHTTPBenchmarkRequest()
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			b.Fatalf("expected status 200, got %d with body %s", recorder.Code, recorder.Body.String())
		}
	}
}
