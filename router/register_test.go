package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testHandler struct{}

func (h *testHandler) Handle(input testInput) testOutput {
	if input.Token == "" {
		return testOutput{Forbidden: &testError{Error: "missing token"}}
	}
	return testOutput{OK: &testSuccess{Message: input.Body.Name + ":" + input.ID}}
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
