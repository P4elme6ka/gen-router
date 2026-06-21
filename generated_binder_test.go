package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeneratedCustomerCreateBinder(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/customers/abc?verbose=true", strings.NewReader(`{"name":"Alice"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "secret")
	req.Header.Set("X-Trace-Id", "trace-1")

	value, err := genRouterBindCustomerCreateHandler(req)
	if err != nil {
		t.Fatalf("generated binder returned error: %v", err)
	}
	input, ok := value.(CustomerCreateInput)
	if !ok {
		t.Fatalf("generated binder returned %T, want CustomerCreateInput", value)
	}
	if input.CustomerID != "abc" {
		t.Fatalf("expected CustomerID abc, got %q", input.CustomerID)
	}
	if input.AuthToken != "secret" {
		t.Fatalf("expected AuthToken secret, got %q", input.AuthToken)
	}
	if input.TraceID != "trace-1" {
		t.Fatalf("expected TraceID trace-1, got %q", input.TraceID)
	}
	if input.Verbose == nil || !*input.Verbose {
		t.Fatalf("expected Verbose true, got %#v", input.Verbose)
	}
	if input.Body.Name != "Alice" {
		t.Fatalf("expected body name Alice, got %q", input.Body.Name)
	}
}

func TestGeneratedCustomerCreateBinderMissingRequiredHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/customers/abc", strings.NewReader(`{"name":"Alice"}`))
	req.Header.Set("Content-Type", "application/json")
	_, err := genRouterBindCustomerCreateHandler(req)
	if err == nil {
		t.Fatal("expected generated binder to fail when required header is missing")
	}
}
