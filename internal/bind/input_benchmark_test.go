package bind

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gen-router/internal/route"
)

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
		if _, err := ParseInput(req, pattern, bindInput{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompilePlanParseInput(b *testing.B) {
	pattern, err := route.Parse("POST /customers/{id}")
	if err != nil {
		b.Fatal(err)
	}
	plan, err := Compile(pattern, bindInput{})
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
