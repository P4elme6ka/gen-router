package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/P4elme6ka/gen-router/src/bind"
	"github.com/P4elme6ka/gen-router/src/route"
)

func newGeneratedBenchmarkRequest() *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/customers/abc?verbose=true", strings.NewReader(`{"name":"Alice"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "secret")
	req.Header.Set("X-Trace-Id", "trace-1")
	return req
}

func BenchmarkGeneratedCustomerCreateBinder(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := newGeneratedBenchmarkRequest()
		if _, err := genRouterBindCustomerCreateHandler(req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompiledPlanCustomerCreateBinder(b *testing.B) {
	pattern, err := route.Parse((CustomerCreateInput{}).EndpointPath())
	if err != nil {
		b.Fatal(err)
	}
	plan, err := bind.Compile(pattern, CustomerCreateInput{})
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := newGeneratedBenchmarkRequest()
		if _, err := plan.Parse(req); err != nil {
			b.Fatal(err)
		}
	}
}
