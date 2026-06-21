package runtime

import "net/http"

type BinderFunc func(*http.Request) (any, error)

type PackageMetadata struct {
	ImportPath  string
	PackageName string
	Dir         string
	Handlers    []HandlerMetadata
}

type HandlerMetadata struct {
	HandlerType string
	InputType   string
	OutputType  string
	Route       RouteMetadata
	Input       InputMetadata
	Output      OutputMetadata
	Binder      BinderMetadata
}

type BinderMetadata struct {
	Symbol string
	Bind   BinderFunc
}

type RouteMetadata struct {
	Method string
	Path   string
}

type InputMetadata struct {
	Body   *InputFieldMetadata
	Fields []InputFieldMetadata
}

type InputFieldMetadata struct {
	GoName string
	Source string
	Name   string
	Type   string
}

type OutputMetadata struct {
	SharedFields []OutputFieldMetadata
	Variants     []OutputVariantMetadata
}

type OutputVariantMetadata struct {
	StatusCode int
	Fields     []OutputFieldMetadata
}

type OutputFieldMetadata struct {
	GoName string
	Source string
	Name   string
	Type   string
}
