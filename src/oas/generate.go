package oas

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/P4elme6ka/gen-router/src/codegen/ir"
)

// GenerateSpec converts discovered handlers to OpenAPI 3.0 spec.
func GenerateSpec(plan *ir.ModulePlan) (*OpenAPISpec, error) {
	spec := &OpenAPISpec{
		OpenAPI: "3.0.0",
		Info: Info{
			Title:   "Generated API",
			Version: "1.0.0",
		},
		Paths:      make(map[string]PathItem),
		Components: Components{Schemas: make(map[string]*Schema)},
	}

	builder, err := newSchemaBuilder(plan)
	if err != nil {
		return nil, err
	}

	for _, pkg := range plan.Packages {
		for _, handler := range pkg.Handlers {
			addHandlerToSpec(spec, builder, pkg.ImportPath, handler)
		}
	}

	for name, schema := range builder.components {
		spec.Components.Schemas[name] = schema
	}

	return spec, nil
}

// addHandlerToSpec converts a single handler to OpenAPI PathItem + Operation.
func addHandlerToSpec(spec *OpenAPISpec, builder *schemaBuilder, currentImportPath string, handler ir.HandlerPlan) {
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
					Name:        field.Name,
					In:          "path",
					Description: field.Description,
					Required:    true,
					Schema:      builder.schemaForTypeString(currentImportPath, field.Type),
				})
			}
		case "query":
			required := !strings.HasPrefix(field.Type, "*")
			op.Parameters = append(op.Parameters, Parameter{
				Name:        field.Name,
				In:          "query",
				Description: field.Description,
				Required:    required,
				Schema:      builder.schemaForTypeString(currentImportPath, field.Type),
			})
		case "header":
			required := !strings.HasPrefix(field.Type, "*")
			op.Parameters = append(op.Parameters, Parameter{
				Name:        field.Name,
				In:          "header",
				Description: field.Description,
				Required:    required,
				Schema:      builder.schemaForTypeString(currentImportPath, field.Type),
			})
		case "cookie":
			required := !strings.HasPrefix(field.Type, "*")
			op.Parameters = append(op.Parameters, Parameter{
				Name:        field.Name,
				In:          "cookie",
				Description: field.Description,
				Required:    required,
				Schema:      builder.schemaForTypeString(currentImportPath, field.Type),
			})
		}
	}

	// Add request body
	if handler.Input.Body != nil {
		required := !strings.HasPrefix(handler.Input.Body.Type, "*")
		op.RequestBody = &RequestBody{
			Description: handler.Input.Body.Description,
			Required:    required,
			Content: map[string]Content{
				"application/json": {
					Schema: builder.bodySchemaForTypeString(currentImportPath, handler.Input.Body.Type),
				},
			},
		}
	}

	// Add responses for each output variant
	for _, variant := range handler.Output.Variants {
		statusCode := strconv.Itoa(variant.StatusCode)
		response := Response{
			Description: responseDescriptionForVariant(variant),
			Headers:     make(map[string]Header),
		}

		// Add body field if present
		bodyField := findFieldBySource(variant.Fields, "body")
		if bodyField != nil {
			response.Content = map[string]Content{
				"application/json": {
					Schema: builder.bodySchemaForTypeString(currentImportPath, bodyField.Type),
				},
			}
		}

		// Add header fields
		for _, field := range variant.Fields {
			if field.Source == "header" {
				response.Headers[field.Name] = Header{
					Description: field.Description,
					Schema:      builder.schemaForTypeString(currentImportPath, field.Type),
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
					Description: field.Description,
					Schema:      builder.schemaForTypeString(currentImportPath, field.Type),
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

func responseDescriptionForVariant(variant ir.OutputVariantPlan) string {
	for _, field := range variant.Fields {
		if field.Source == "body" && field.Description != "" {
			return field.Description
		}
	}
	for _, field := range variant.Fields {
		if field.Description != "" {
			return field.Description
		}
	}
	return fmt.Sprintf("Status %d response", variant.StatusCode)
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
