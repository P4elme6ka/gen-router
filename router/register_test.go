package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type ctxKey string

type testHandler struct{}

func (h *testHandler) Handle(ctx context.Context, input testInput) testOutput {
	prefix, _ := ctx.Value(ctxKey("prefix")).(string)
	if input.Token == "" {
		return testOutput{Forbidden: &testError{Error: "missing token"}}
	}
	return testOutput{OK: &testSuccess{Message: prefix + input.Body.Name + ":" + input.ID}}
}

func (h *testHandler) I() testInput {
	return testInput{}
}

type testInput struct {
	ID    string `gen-router:"in:path;name:id"`
	Token string `gen-router:"in:header;name:X-Token"`
	Body  struct {
		Name string `json:"name"`
	} `gen-router:"in:body"`
}

func (i testInput) EndpointPath() string {
	return "POST /items/{id}"
}

func (i testInput) GetEndpointPath() string {
	return "GET /items/{id}"
}

type testOutput struct {
	OK        *testSuccess `gen-router:"response:200;in:body"`
	Forbidden *testError   `gen-router:"response:403;in:body"`
}

type testSuccess struct {
	Message string `json:"message"`
}

type testError struct {
	Error string `json:"error"`
}

func TestRegister_EndToEnd(t *testing.T) {
	r := New()
	if err := Register(r, &testHandler{}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/items/abc", strings.NewReader(`{"name":"widget"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Token", "secret")
	recorder := httptest.NewRecorder()

	r.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var body testSuccess
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body.Message != "widget:abc" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestRegister_BadInputReturns400(t *testing.T) {
	r := New()
	if err := Register(r, &testHandler{}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/items/abc", strings.NewReader(`{"name":"widget"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	r.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

type middlewareInput struct {
	ID string `gen-router:"in:path;name:id"`
}

func (i middlewareInput) EndpointPath() string {
	return "POST /items/{id}"
}

type prefixMiddleware struct {
	prefix string
	order  *[]string
}

func (m *prefixMiddleware) Handle(ctx context.Context, input middlewareInput, next Next[testOutput]) testOutput {
	*m.order = append(*m.order, m.prefix+":"+input.ID)
	current, _ := ctx.Value(ctxKey("prefix")).(string)
	ctx = context.WithValue(ctx, ctxKey("prefix"), current+m.prefix)
	return next(ctx)
}

func (m *prefixMiddleware) I() middlewareInput {
	return middlewareInput{}
}

func TestUse_MiddlewaresRunInRegistrationOrderAndPropagateContext(t *testing.T) {
	r := New()
	order := []string{}
	if err := Use(r, "POST /items", &prefixMiddleware{prefix: "A", order: &order}); err != nil {
		t.Fatalf("Use returned error: %v", err)
	}
	if err := Use(r, "POST /items/{id}", &prefixMiddleware{prefix: "B", order: &order}); err != nil {
		t.Fatalf("Use returned error: %v", err)
	}
	if err := Register(r, &testHandler{}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/items/abc", strings.NewReader(`{"name":"widget"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Token", "secret")
	recorder := httptest.NewRecorder()

	r.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if len(order) != 2 || order[0] != "A:abc" || order[1] != "B:abc" {
		t.Fatalf("unexpected middleware order: %#v", order)
	}

	var body testSuccess
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body.Message != "ABwidget:abc" {
		t.Fatalf("unexpected middleware-influenced body: %#v", body)
	}
}

func TestUse_PathScopeDoesNotMatchDifferentPrefix(t *testing.T) {
	r := New()
	order := []string{}
	if err := Use(r, "POST /admin", &prefixMiddleware{prefix: "X", order: &order}); err != nil {
		t.Fatalf("Use returned error: %v", err)
	}
	if err := Register(r, &testHandler{}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/items/abc", strings.NewReader(`{"name":"widget"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Token", "secret")
	recorder := httptest.NewRecorder()

	r.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if len(order) != 0 {
		t.Fatalf("expected no middleware execution, got %#v", order)
	}
}

func TestUse_PathOnlyScopeMatchesAnyMethod(t *testing.T) {
	r := New()
	order := []string{}
	if err := Use(r, "/items", &prefixMiddleware{prefix: "P", order: &order}); err != nil {
		t.Fatalf("Use returned error: %v", err)
	}
	if err := Register(r, &testHandler{}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/items/abc", strings.NewReader(`{"name":"widget"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Token", "secret")
	recorder := httptest.NewRecorder()

	r.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if len(order) != 1 || order[0] != "P:abc" {
		t.Fatalf("expected path-only middleware to run, got %#v", order)
	}
}

func TestUse_MethodSpecificScopeDoesNotMatchOtherMethod(t *testing.T) {
	r := New()
	order := []string{}
	if err := Use(r, "GET /items", &prefixMiddleware{prefix: "G", order: &order}); err != nil {
		t.Fatalf("Use returned error: %v", err)
	}
	if err := Register(r, &testHandler{}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/items/abc", strings.NewReader(`{"name":"widget"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Token", "secret")
	recorder := httptest.NewRecorder()

	r.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if len(order) != 0 {
		t.Fatalf("expected GET-scoped middleware not to run on POST, got %#v", order)
	}
}
