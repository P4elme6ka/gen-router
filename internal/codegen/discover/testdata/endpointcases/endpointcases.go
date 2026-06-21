package endpointcases

import (
	"context"
	"fmt"
)

const method = "POST"
const basePath = "/customers"
const resourcePath = basePath + "/{id}"

var versionedPath = "/v1" + resourcePath

type LiteralHandler struct{}

func (LiteralHandler) Handle(ctx context.Context, input LiteralInput) struct{} {
	_ = ctx
	return struct{}{}
}
func (LiteralHandler) I() LiteralInput { return LiteralInput{} }

type LiteralInput struct{}

func (LiteralInput) EndpointPath() string {
	return "GET /literal"
}

type ConcatHandler struct{}

func (ConcatHandler) Handle(ctx context.Context, input ConcatInput) struct{} {
	_ = ctx
	return struct{}{}
}
func (ConcatHandler) I() ConcatInput { return ConcatInput{} }

type ConcatInput struct{}

func (ConcatInput) EndpointPath() string {
	return method + " " + resourcePath
}

type SprintfHandler struct{}

func (SprintfHandler) Handle(ctx context.Context, input SprintfInput) struct{} {
	_ = ctx
	return struct{}{}
}
func (SprintfHandler) I() SprintfInput { return SprintfInput{} }

type SprintfInput struct{}

func (SprintfInput) EndpointPath() string {
	return fmt.Sprintf("%s %s", method, versionedPath)
}

type NestedHandler struct{}

func (NestedHandler) Handle(ctx context.Context, input NestedInput) struct{} {
	_ = ctx
	return struct{}{}
}
func (NestedHandler) I() NestedInput { return NestedInput{} }

type NestedInput struct{}

func (NestedInput) EndpointPath() string {
	return fmt.Sprintf("%s %s", "PATCH", fmt.Sprintf("%s/%s", "/items", "{id}"))
}
