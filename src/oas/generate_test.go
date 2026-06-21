package oas

import (
	"testing"

	"github.com/P4elme6ka/gen-router/src/codegen/discover"
)

func TestGenerateSpec_FillsObjectFieldsFromStructTypes(t *testing.T) {
	plan, err := discover.LoadModulePlan(discover.Config{Patterns: []string{"./testdata/schemaapi"}})
	if err != nil {
		t.Fatalf("LoadModulePlan returned error: %v", err)
	}

	spec, err := GenerateSpec(plan)
	if err != nil {
		t.Fatalf("GenerateSpec returned error: %v", err)
	}

	requestSchema := spec.Components.Schemas["createRequest"]
	if requestSchema == nil {
		t.Fatal("expected createRequest schema to be generated")
	}
	if requestSchema.Type != "object" {
		t.Fatalf("expected createRequest schema type object, got %q", requestSchema.Type)
	}
	assertPropertyType(t, requestSchema, "id", "string", "uuid")
	assertPropertyType(t, requestSchema, "name", "string", "")
	assertPropertyType(t, requestSchema, "count", "integer", "int32")
	assertArrayItemType(t, requestSchema, "tags", "string")
	assertMapValueType(t, requestSchema, "meta", "string")
	assertPropertyRef(t, requestSchema, "nested", "#/components/schemas/child")
	assertPropertyType(t, requestSchema, "createdAt", "string", "date-time")
	assertRequired(t, requestSchema, "id", "name", "nested", "createdAt")
	assertNotRequired(t, requestSchema, "count", "tags", "meta")

	childSchema := spec.Components.Schemas["child"]
	if childSchema == nil {
		t.Fatal("expected child schema to be generated")
	}
	assertPropertyType(t, childSchema, "enabled", "boolean", "")
	assertRequired(t, childSchema, "enabled")

	responseSchema := spec.Components.Schemas["createResponse"]
	if responseSchema == nil {
		t.Fatal("expected createResponse schema to be generated")
	}
	assertPropertyType(t, responseSchema, "message", "string", "")
	assertPropertyRef(t, responseSchema, "nested", "#/components/schemas/child")

	operation := spec.Paths["/items/{id}"].Post
	if operation == nil || operation.RequestBody == nil {
		t.Fatal("expected POST /items/{id} request body to be generated")
	}
	assertParameterDescription(t, operation.Parameters, "id", "path", "Item identifier")
	if got := operation.RequestBody.Description; got != "Payload for item creation" {
		t.Fatalf("expected request body description %q, got %q", "Payload for item creation", got)
	}
	if got := operation.RequestBody.Content["application/json"].Schema.Ref; got != "#/components/schemas/createRequest" {
		t.Fatalf("expected request body ref to createRequest, got %q", got)
	}
	response := operation.Responses["200"]
	if got := response.Description; got != "Created item response" {
		t.Fatalf("expected response description %q, got %q", "Created item response", got)
	}
	if got := response.Content["application/json"].Schema.Ref; got != "#/components/schemas/createResponse" {
		t.Fatalf("expected response body ref to createResponse, got %q", got)
	}
	if got := response.Headers["X-Request-Id"].Description; got != "Tracing request identifier" {
		t.Fatalf("expected response header description %q, got %q", "Tracing request identifier", got)
	}
}

func assertPropertyType(t *testing.T, schema *Schema, name, wantType, wantFormat string) {
	t.Helper()
	prop := schema.Properties[name]
	if prop == nil {
		t.Fatalf("expected property %q to exist", name)
	}
	if prop.Type != wantType {
		t.Fatalf("expected property %q type %q, got %q", name, wantType, prop.Type)
	}
	if prop.Format != wantFormat {
		t.Fatalf("expected property %q format %q, got %q", name, wantFormat, prop.Format)
	}
}

func assertArrayItemType(t *testing.T, schema *Schema, name, wantItemType string) {
	t.Helper()
	prop := schema.Properties[name]
	if prop == nil {
		t.Fatalf("expected property %q to exist", name)
	}
	if prop.Type != "array" {
		t.Fatalf("expected property %q to be array, got %q", name, prop.Type)
	}
	if prop.Items == nil || prop.Items.Type != wantItemType {
		t.Fatalf("expected property %q items type %q, got %#v", name, wantItemType, prop.Items)
	}
}

func assertMapValueType(t *testing.T, schema *Schema, name, wantValueType string) {
	t.Helper()
	prop := schema.Properties[name]
	if prop == nil {
		t.Fatalf("expected property %q to exist", name)
	}
	if prop.Type != "object" {
		t.Fatalf("expected property %q to be object, got %q", name, prop.Type)
	}
	additional, ok := prop.AdditionalProperties.(*Schema)
	if !ok || additional == nil {
		t.Fatalf("expected property %q to have schema additionalProperties, got %#v", name, prop.AdditionalProperties)
	}
	if additional.Type != wantValueType {
		t.Fatalf("expected property %q additionalProperties type %q, got %q", name, wantValueType, additional.Type)
	}
}

func assertPropertyRef(t *testing.T, schema *Schema, name, wantRef string) {
	t.Helper()
	prop := schema.Properties[name]
	if prop == nil {
		t.Fatalf("expected property %q to exist", name)
	}
	if prop.Ref != wantRef {
		t.Fatalf("expected property %q ref %q, got %q", name, wantRef, prop.Ref)
	}
}

func assertParameterDescription(t *testing.T, parameters []Parameter, name, in, wantDescription string) {
	t.Helper()
	for _, parameter := range parameters {
		if parameter.Name == name && parameter.In == in {
			if parameter.Description != wantDescription {
				t.Fatalf("expected parameter %s in %s to have description %q, got %q", name, in, wantDescription, parameter.Description)
			}
			return
		}
	}
	t.Fatalf("expected parameter %s in %s to exist", name, in)
}

func assertRequired(t *testing.T, schema *Schema, names ...string) {
	t.Helper()
	for _, name := range names {
		if !containsString(schema.Required, name) {
			t.Fatalf("expected %q to be required, got %v", name, schema.Required)
		}
	}
}

func assertNotRequired(t *testing.T, schema *Schema, names ...string) {
	t.Helper()
	for _, name := range names {
		if containsString(schema.Required, name) {
			t.Fatalf("expected %q to be optional, got required list %v", name, schema.Required)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
