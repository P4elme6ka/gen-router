package genbench

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/P4elme6ka/gen-router/src/bind"
	"github.com/P4elme6ka/gen-router/src/route"
)

type benchmarkBindInput struct {
	CustomerID string            `gen-router:"in:path;name:id"`
	AuthToken  string            `gen-router:"in:header;name:X-Auth-Token"`
	Verbose    *bool             `gen-router:"in:query;name:verbose"`
	Tags       []string          `gen-router:"in:query;name:tag"`
	Body       benchmarkBindBody `gen-router:"in:body"`
}

type benchmarkBindBody struct {
	Name string `json:"name"`
}

func newBenchmarkInputRequest() *http.Request {
	req := httptest.NewRequest("POST", "/customers/42?verbose=true&tag=one&tag=two", strings.NewReader(`{"name":"Alice"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "secret")
	return req
}

func BenchmarkParseInput(b *testing.B) {
	pattern, err := route.Parse("POST /customers/{id}")
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := newBenchmarkInputRequest()
		if _, err := bind.ParseInput(req, pattern, benchmarkBindInput{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompilePlanParseInput(b *testing.B) {
	pattern, err := route.Parse("POST /customers/{id}")
	if err != nil {
		b.Fatal(err)
	}
	plan, err := bind.Compile(pattern, benchmarkBindInput{})
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := newBenchmarkInputRequest()
		if _, err := plan.Parse(req); err != nil {
			b.Fatal(err)
		}
	}
}
