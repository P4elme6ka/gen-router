package oas

// OpenAPI 3.0.0 structures

type OpenAPISpec struct {
	OpenAPI      string                 `json:"openapi"`
	Info         Info                   `json:"info"`
	Paths        map[string]PathItem    `json:"paths"`
	Components   Components             `json:"components,omitempty"`
	Tags         []Tag                  `json:"tags,omitempty"`
	Servers      []Server               `json:"servers,omitempty"`
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty"`
}

type Info struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

type PathItem struct {
	Get        *Operation  `json:"get,omitempty"`
	Post       *Operation  `json:"post,omitempty"`
	Put        *Operation  `json:"put,omitempty"`
	Delete     *Operation  `json:"delete,omitempty"`
	Patch      *Operation  `json:"patch,omitempty"`
	Head       *Operation  `json:"head,omitempty"`
	Options    *Operation  `json:"options,omitempty"`
	Trace      *Operation  `json:"trace,omitempty"`
	Parameters []Parameter `json:"parameters,omitempty"`
}

type Operation struct {
	OperationID string              `json:"operationId,omitempty"`
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
}

type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"` // "path", "query", "header", "cookie"
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required"`
	Schema      *Schema `json:"schema,omitempty"`
	Style       string  `json:"style,omitempty"`
	Explode     *bool   `json:"explode,omitempty"`
}

type RequestBody struct {
	Description string             `json:"description,omitempty"`
	Required    bool               `json:"required"`
	Content     map[string]Content `json:"content"`
}

type Content struct {
	Schema *Schema `json:"schema,omitempty"`
}

type Response struct {
	Description string             `json:"description"`
	Content     map[string]Content `json:"content,omitempty"`
	Headers     map[string]Header  `json:"headers,omitempty"`
}

type Header struct {
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Description          string             `json:"description,omitempty"`
	Ref                  string             `json:"$ref,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Enum                 []interface{}      `json:"enum,omitempty"`
	Default              interface{}        `json:"default,omitempty"`
	Example              interface{}        `json:"example,omitempty"`
	AdditionalProperties interface{}        `json:"additionalProperties,omitempty"`
}

type Components struct {
	Schemas map[string]*Schema `json:"schemas"`
}

type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

type ExternalDocumentation struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}
