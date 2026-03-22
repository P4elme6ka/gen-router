package render

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type renderSuccess struct {
	Message string `json:"message"`
}

type renderError struct {
	Error string `json:"error"`
}

type renderOutput struct {
	RequestID string         `gen-router:"response:200;in:header;name:X-Request-Id"`
	Success   *renderSuccess `gen-router:"response:200;in:body"`
	BadInput  *renderError   `gen-router:"response:400;in:body"`
}

type sharedRenderOutput struct {
	TraceID  string         `gen-router:"in:header;name:X-Trace-Id"`
	Success  *renderSuccess `gen-router:"response:200;in:body"`
	BadInput *renderError   `gen-router:"response:400;in:body"`
}

func TestWriteOutput_WritesFirstSuitableVariant(t *testing.T) {
	recorder := httptest.NewRecorder()
	output := renderOutput{
		RequestID: "req-1",
		Success:   &renderSuccess{Message: "ok"},
		BadInput:  &renderError{Error: "bad"},
	}

	if err := WriteOutput(recorder, output); err != nil {
		t.Fatalf("WriteOutput returned error: %v", err)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if recorder.Header().Get("X-Request-Id") != "req-1" {
		t.Fatalf("expected X-Request-Id header, got %q", recorder.Header().Get("X-Request-Id"))
	}

	var body renderSuccess
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body.Message != "ok" {
		t.Fatalf("expected message ok, got %q", body.Message)
	}
}

func TestWriteOutput_SelectsErrorVariantWhenSuccessMissing(t *testing.T) {
	recorder := httptest.NewRecorder()
	output := renderOutput{
		BadInput: &renderError{Error: "nope"},
	}

	if err := WriteOutput(recorder, output); err != nil {
		t.Fatalf("WriteOutput returned error: %v", err)
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestCompilePlan_WriteUsesPrecompiledVariantLogic(t *testing.T) {
	plan, err := Compile(renderOutput{})
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	recorder := httptest.NewRecorder()
	output := renderOutput{
		RequestID: "req-compiled",
		Success:   &renderSuccess{Message: "compiled"},
	}

	if err := plan.Write(recorder, output); err != nil {
		t.Fatalf("plan.Write returned error: %v", err)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if recorder.Header().Get("X-Request-Id") != "req-compiled" {
		t.Fatalf("expected compiled header, got %q", recorder.Header().Get("X-Request-Id"))
	}

	var body renderSuccess
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body.Message != "compiled" {
		t.Fatalf("expected message compiled, got %q", body.Message)
	}
}

func TestWriteOutput_AppliesSharedHeadersToSuccessVariant(t *testing.T) {
	recorder := httptest.NewRecorder()
	output := sharedRenderOutput{
		TraceID: "trace-123",
		Success: &renderSuccess{Message: "ok"},
	}

	if err := WriteOutput(recorder, output); err != nil {
		t.Fatalf("WriteOutput returned error: %v", err)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if recorder.Header().Get("X-Trace-Id") != "trace-123" {
		t.Fatalf("expected shared X-Trace-Id header, got %q", recorder.Header().Get("X-Trace-Id"))
	}
}

func TestWriteOutput_AppliesSharedHeadersToErrorVariant(t *testing.T) {
	recorder := httptest.NewRecorder()
	output := sharedRenderOutput{
		TraceID:  "trace-err",
		BadInput: &renderError{Error: "bad"},
	}

	if err := WriteOutput(recorder, output); err != nil {
		t.Fatalf("WriteOutput returned error: %v", err)
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
	if recorder.Header().Get("X-Trace-Id") != "trace-err" {
		t.Fatalf("expected shared X-Trace-Id header, got %q", recorder.Header().Get("X-Trace-Id"))
	}
}

func TestWriteOutput_SharedFieldsDoNotSelectVariant(t *testing.T) {
	recorder := httptest.NewRecorder()
	output := sharedRenderOutput{TraceID: "trace-only"}

	err := WriteOutput(recorder, output)
	if err == nil {
		t.Fatal("expected WriteOutput to fail when only shared fields are set")
	}
}
