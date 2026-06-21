package genbench

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/P4elme6ka/gen-router/router"
)

type benchmarkRouterHandler struct{}

type benchmarkRouterInput struct {
	CustomerID string              `gen-router:"in:path;name:id"`
	AuthToken  string              `gen-router:"in:header;name:X-Auth-Token"`
	TraceID    string              `gen-router:"in:header;name:X-Trace-Id"`
	Verbose    *bool               `gen-router:"in:query;name:verbose"`
	Body       benchmarkRouterBody `gen-router:"in:body"`
}

func (benchmarkRouterInput) EndpointPath() string {
	return "POST /customers/{id}"
}

type benchmarkRouterBody struct {
	Name string `json:"name"`
}

type benchmarkRouterOutput struct {
	RequestID    string                    `gen-router:"in:header;name:X-Request-Id"`
	TraceID      string                    `gen-router:"in:header;name:X-Trace-Id"`
	StatusOK     *benchmarkRouterSuccess   `gen-router:"response:200;in:body"`
	BadRequest   *benchmarkRouterErrorBody `gen-router:"response:400;in:body"`
	Unauthorized *benchmarkRouterErrorBody `gen-router:"response:403;in:body"`
}

type benchmarkRouterSuccess struct {
	Message string `json:"message"`
}

type benchmarkRouterErrorBody struct {
	Error string `json:"error"`
}

func (h *benchmarkRouterHandler) Handle(ctx context.Context, input benchmarkRouterInput) benchmarkRouterOutput {
	_ = ctx
	if strings.TrimSpace(input.AuthToken) == "" {
		return benchmarkRouterOutput{Unauthorized: &benchmarkRouterErrorBody{Error: "missing X-Auth-Token header"}}
	}
	if strings.TrimSpace(input.CustomerID) == "" {
		return benchmarkRouterOutput{BadRequest: &benchmarkRouterErrorBody{Error: "missing customer id"}}
	}
	if strings.TrimSpace(input.Body.Name) == "" {
		return benchmarkRouterOutput{BadRequest: &benchmarkRouterErrorBody{Error: "name is required"}}
	}
	return benchmarkRouterOutput{
		StatusOK:  &benchmarkRouterSuccess{Message: fmt.Sprintf("customer %s created", input.CustomerID)},
		TraceID:   input.TraceID,
		RequestID: fmt.Sprintf("req-%s", input.CustomerID),
	}
}

func (h *benchmarkRouterHandler) I() benchmarkRouterInput {
	return benchmarkRouterInput{}
}

func newMainBenchmarkRequest() *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/customers/abc?verbose=true", strings.NewReader(`{"name":"Alice"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "secret")
	req.Header.Set("X-Trace-Id", "trace-1")
	return req
}

func BenchmarkRouterServeHTTP(b *testing.B) {
	r := routerForBenchmark(b)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := newMainBenchmarkRequest()
		recorder := httptest.NewRecorder()
		r.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			b.Fatalf("expected status 200, got %d with body %s", recorder.Code, recorder.Body.String())
		}
	}
}

func routerForBenchmark(b *testing.B) http.Handler {
	b.Helper()
	r := router.New()
	if err := router.Register(r, &benchmarkRouterHandler{}); err != nil {
		b.Fatal(err)
	}
	return r
}
