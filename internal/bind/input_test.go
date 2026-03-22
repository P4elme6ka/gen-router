package bind

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"gen-router/internal/route"
)

type bindInput struct {
	ID      int      `gen-router:"in:path;name:id"`
	Token   string   `gen-router:"in:header;name:X-Auth-Token"`
	Verbose *bool    `gen-router:"in:query;name:verbose"`
	Tags    []string `gen-router:"in:query;name:tag"`
	Body    struct {
		Name string `json:"name"`
	} `gen-router:"in:body"`
}

func TestParseInput_BindsAllSupportedSources(t *testing.T) {
	pattern, err := route.Parse("POST /customers/{id}")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/customers/42?verbose=true&tag=one&tag=two", strings.NewReader(`{"name":"Alice"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "secret")

	input, err := ParseInput(req, pattern, bindInput{})
	if err != nil {
		t.Fatalf("ParseInput returned error: %v", err)
	}
	if input.ID != 42 {
		t.Fatalf("expected ID 42, got %d", input.ID)
	}
	if input.Token != "secret" {
		t.Fatalf("expected token secret, got %q", input.Token)
	}
	if input.Verbose == nil || !*input.Verbose {
		t.Fatalf("expected verbose=true, got %#v", input.Verbose)
	}
	if len(input.Tags) != 2 || input.Tags[0] != "one" || input.Tags[1] != "two" {
		t.Fatalf("expected tags [one two], got %#v", input.Tags)
	}
	if input.Body.Name != "Alice" {
		t.Fatalf("expected body name Alice, got %q", input.Body.Name)
	}
}

func TestParseInput_RejectsMissingRequiredHeader(t *testing.T) {
	pattern, err := route.Parse("POST /customers/{id}")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/customers/42", strings.NewReader(`{"name":"Alice"}`))
	req.Header.Set("Content-Type", "application/json")

	_, err = ParseInput(req, pattern, bindInput{})
	if err == nil {
		t.Fatal("expected ParseInput to fail when required header is missing")
	}
}

func TestCompilePlan_ParseReusesCompiledInputLogic(t *testing.T) {
	pattern, err := route.Parse("POST /customers/{id}")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Compile(pattern, bindInput{})
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	req := httptest.NewRequest("POST", "/customers/7?tag=a&tag=b", strings.NewReader(`{"name":"Bob"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "compiled-secret")

	input, err := plan.Parse(req)
	if err != nil {
		t.Fatalf("plan.Parse returned error: %v", err)
	}
	if input.ID != 7 {
		t.Fatalf("expected ID 7, got %d", input.ID)
	}
	if input.Token != "compiled-secret" {
		t.Fatalf("expected token compiled-secret, got %q", input.Token)
	}
	if input.Body.Name != "Bob" {
		t.Fatalf("expected body name Bob, got %q", input.Body.Name)
	}
	if len(input.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %#v", input.Tags)
	}
}

type bindUUIDInput struct {
	PathID  uuid.UUID  `gen-router:"in:path;name:id"`
	QueryID *uuid.UUID `gen-router:"in:query;name:query_id"`
}

func TestParseInput_BindsUUIDFromPathAndQuery(t *testing.T) {
	pattern, err := route.Parse("GET /resources/{id}")
	if err != nil {
		t.Fatal(err)
	}

	pathID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	queryID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	req := httptest.NewRequest("GET", "/resources/"+pathID.String()+"?query_id="+queryID.String(), nil)

	input, err := ParseInput(req, pattern, bindUUIDInput{})
	if err != nil {
		t.Fatalf("ParseInput returned error: %v", err)
	}
	if input.PathID != pathID {
		t.Fatalf("expected path UUID %s, got %s", pathID, input.PathID)
	}
	if input.QueryID == nil || *input.QueryID != queryID {
		t.Fatalf("expected query UUID %s, got %#v", queryID, input.QueryID)
	}
}

func TestParseInput_RejectsInvalidUUID(t *testing.T) {
	pattern, err := route.Parse("GET /resources/{id}")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/resources/not-a-uuid?query_id=also-bad", nil)
	_, err = ParseInput(req, pattern, bindUUIDInput{})
	if err == nil {
		t.Fatal("expected ParseInput to fail for invalid UUID values")
	}
}
