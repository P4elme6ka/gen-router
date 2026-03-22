package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/P4elme6ka/gen-router/router"
)

func newMainBenchmarkRequest() *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/customers/abc?verbose=true", strings.NewReader(`{"name":"Alice"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "secret")
	req.Header.Set("X-Trace-Id", "trace-1")
	return req
}

func BenchmarkGeneratedCustomerCreateBinder(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := newMainBenchmarkRequest()
		value, err := genRouterBindCustomerCreateHandler(req)
		if err != nil {
			b.Fatal(err)
		}
		if _, ok := value.(CustomerCreateInput); !ok {
			b.Fatalf("generated binder returned %T, want CustomerCreateInput", value)
		}
	}
}

func BenchmarkCustomerCreateHandleAfterGeneratedBind(b *testing.B) {
	handler := &CustomerCreateHandler{}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := newMainBenchmarkRequest()
		value, err := genRouterBindCustomerCreateHandler(req)
		if err != nil {
			b.Fatal(err)
		}
		input, ok := value.(CustomerCreateInput)
		if !ok {
			b.Fatalf("generated binder returned %T, want CustomerCreateInput", value)
		}
		output := handler.Handle(context.Background(), input)
		if output.StatusOK == nil {
			b.Fatalf("expected success output, got %#v", output)
		}
	}
}

func BenchmarkCustomerCreateServeHTTP(b *testing.B) {
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
	if err := router.Register(r, &CustomerCreateHandler{}); err != nil {
		b.Fatal(err)
	}
	return r
}
