package oas

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/P4elme6ka/gen-router/internal/codegen/ir"
)

// GenerateSpec converts discovered handlers to OpenAPI 3.0 spec.
func GenerateSpec(plan *ir.ModulePlan) *OpenAPISpec {
	spec := &OpenAPISpec{
		OpenAPI: "3.0.0",
		Info: Info{
			Title:   "Generated API",
			Version: "1.0.0",
		},
		Paths:      make(map[string]PathItem),
		Components: Components{Schemas: make(map[string]*Schema)},
	}

	// Collect all types we'll need schemas for
	typeSchemas := make(map[string]*Schema)

	for _, pkg := range plan.Packages {
		for _, handler := range pkg.Handlers {
			// Generate schemas for input/output types
			if handler.Input.Body != nil {
				typeName := extractTypeName(handler.Input.Body.Type)
				if _, exists := typeSchemas[typeName]; !exists {
					typeSchemas[typeName] = generateSchemaForType(typeName)
				}
			}

			for _, variant := range handler.Output.Variants {
				for _, field := range variant.Fields {
					if field.Source == "body" {
						typeName := extractTypeName(field.Type)
						if _, exists := typeSchemas[typeName]; !exists {
							typeSchemas[typeName] = generateSchemaForType(typeName)
						}
					}
				}
			}

			// Add handler to paths
			addHandlerToSpec(spec, handler)
		}
	}

	// Add all type schemas to components
	for typeName, schema := range typeSchemas {
		spec.Components.Schemas[typeName] = schema
	}

	return spec
}

// addHandlerToSpec converts a single handler to OpenAPI PathItem + Operation.
func addHandlerToSpec(spec *OpenAPISpec, handler ir.HandlerPlan) {
	pathKey := handler.Route.Path
	methodLower := strings.ToLower(handler.Route.Method)

	// Get or create PathItem
	pathItem := spec.Paths[pathKey]
	if pathItem.Parameters == nil {
		pathItem.Parameters = []Parameter{}
	}

	// Create operation
	op := &Operation{
		OperationID: fmt.Sprintf("%s%s", strings.ToUpper(methodLower), camelCase(pathKey)),
		Summary:     handler.HandlerType,
		Responses:   make(map[string]Response),
	}

	// Add parameters from input fields
	pathParams := extractPathParams(handler.Route.Path)
	for _, field := range handler.Input.Fields {
		switch field.Source {
		case "path":
			if _, isPathParam := pathParams[field.Name]; isPathParam {
				op.Parameters = append(op.Parameters, Parameter{
					Name:     field.Name,
					In:       "path",
					Required: true,
					Schema:   schemaForType(field.Type),
				})
			}
		case "query":
			required := !strings.HasPrefix(field.Type, "*")
			op.Parameters = append(op.Parameters, Parameter{
				Name:     field.Name,
				In:       "query",
				Required: required,
				Schema:   schemaForType(field.Type),
			})
		case "header":
			required := !strings.HasPrefix(field.Type, "*")
			op.Parameters = append(op.Parameters, Parameter{
				Name:     field.Name,
				In:       "header",
				Required: required,
				Schema:   schemaForType(field.Type),
			})
		case "cookie":
			required := !strings.HasPrefix(field.Type, "*")
			op.Parameters = append(op.Parameters, Parameter{
				Name:     field.Name,
				In:       "cookie",
				Required: required,
				Schema:   schemaForType(field.Type),
			})
		}
	}

	// Add request body
	if handler.Input.Body != nil {
		typeName := extractTypeName(handler.Input.Body.Type)
		required := !strings.HasPrefix(handler.Input.Body.Type, "*")
		op.RequestBody = &RequestBody{
			Required: required,
			Content: map[string]Content{
				"application/json": {
					Schema: &Schema{Ref: "#/components/schemas/" + typeName},
				},
			},
		}
	}

	// Add responses for each output variant
	for _, variant := range handler.Output.Variants {
		statusCode := strconv.Itoa(variant.StatusCode)
		response := Response{
			Description: fmt.Sprintf("Status %d response", variant.StatusCode),
			Headers:     make(map[string]Header),
		}

		// Add body field if present
		bodyField := findFieldBySource(variant.Fields, "body")
		if bodyField != nil {
			typeName := extractTypeName(bodyField.Type)
			response.Content = map[string]Content{
				"application/json": {
					Schema: &Schema{Ref: "#/components/schemas/" + typeName},
				},
			}
		}

		// Add header fields
		for _, field := range variant.Fields {
			if field.Source == "header" {
				response.Headers[field.Name] = Header{
					Schema: schemaForType(field.Type),
				}
			}
		}

		op.Responses[statusCode] = response
	}

	// Add shared headers to all responses
	for _, field := range handler.Output.SharedFields {
		if field.Source == "header" {
			for statusCode := range op.Responses {
				response := op.Responses[statusCode]
				if response.Headers == nil {
					response.Headers = make(map[string]Header)
				}
				response.Headers[field.Name] = Header{
					Schema: schemaForType(field.Type),
				}
				op.Responses[statusCode] = response
			}
		}
	}

	// Set operation on path item
	switch strings.ToUpper(handler.Route.Method) {
	case "GET":
		pathItem.Get = op
	case "POST":
		pathItem.Post = op
	case "PUT":
		pathItem.Put = op
	case "DELETE":
		pathItem.Delete = op
	case "PATCH":
		pathItem.Patch = op
	case "HEAD":
		pathItem.Head = op
	case "OPTIONS":
		pathItem.Options = op
	case "TRACE":
		pathItem.Trace = op
	}

	spec.Paths[pathKey] = pathItem
}

// schemaForType returns a basic Schema for a Go type string.
func schemaForType(typeName string) *Schema {
	typeName = strings.TrimPrefix(typeName, "*")
	typeName = strings.TrimPrefix(typeName, "[]")

	switch typeName {
	case "string":
		return &Schema{Type: "string"}
	case "bool":
		return &Schema{Type: "boolean"}
	case "int", "int8", "int16", "int32":
		return &Schema{Type: "integer", Format: "int32"}
	case "int64":
		return &Schema{Type: "integer", Format: "int64"}
	case "uint", "uint8", "uint16", "uint32":
		return &Schema{Type: "integer", Format: "uint32"}
	case "uint64":
		return &Schema{Type: "integer", Format: "uint64"}
	case "float32":
		return &Schema{Type: "number", Format: "float"}
	case "float64":
		return &Schema{Type: "number", Format: "double"}
	case "uuid.UUID", "github.com/google/uuid.UUID":
		return &Schema{Type: "string", Format: "uuid"}
	default:
		// Assume it's a struct reference
		return &Schema{Ref: "#/components/schemas/" + typeName}
	}
}

// generateSchemaForType creates a schema for a named type.
// This is a placeholder; proper implementation would inspect struct fields.
func generateSchemaForType(typeName string) *Schema {
	return &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
		Required:   []string{},
	}
}

// extractTypeName removes package prefix and pointer/slice markers.
func extractTypeName(typeStr string) string {
	// Remove leading *
	typeStr = strings.TrimPrefix(typeStr, "*")
	// Remove leading []
	typeStr = strings.TrimPrefix(typeStr, "[]")
	// Extract just the type name (e.g., "main.CustomerCreateBody" -> "CustomerCreateBody")
	if idx := strings.LastIndex(typeStr, "."); idx != -1 {
		return typeStr[idx+1:]
	}
	return typeStr
}

// extractPathParams returns a map of path parameter names from a route path.
func extractPathParams(path string) map[string]bool {
	params := make(map[string]bool)
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for _, segment := range segments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			paramName := strings.Trim(segment, "{}")
			params[paramName] = true
		}
	}
	return params
}

// findFieldBySource finds the first field with a given source type.
func findFieldBySource(fields []ir.OutputFieldPlan, source string) *ir.OutputFieldPlan {
	for i := range fields {
		if fields[i].Source == source {
			return &fields[i]
		}
	}
	return nil
}

// camelCase converts a path like "/customers/{id}" to "CustomersId".
func camelCase(path string) string {
	var result strings.Builder
	capitalize := true
	for _, ch := range path {
		if ch == '/' || ch == '{' || ch == '}' {
			capitalize = true
		} else if capitalize && ch >= 'a' && ch <= 'z' {
			result.WriteRune(ch - 32)
			capitalize = false
		} else if !capitalize && ch >= 'A' && ch <= 'Z' {
			result.WriteRune(ch + 32)
		} else if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' {
			result.WriteRune(ch)
			capitalize = false
		}
	}
	return result.String()
}
