package router

import (
	"context"
	"net/http"
)

type Input interface {
	EndpointPath() string
}

type Output interface{}

type Handler[I Input, O Output] interface {
	Handle(ctx context.Context, input I) O
	I() I
}

type Next[O Output] func(context.Context) O

type Middleware[I Input, O Output] interface {
	Handle(ctx context.Context, input I, next Next[O]) O
	I() I
}

type Router struct {
	mux         http.ServeMux
	middlewares []registeredMiddleware
}

type registeredMiddleware struct {
	method string
	path   string
	match  func(requestPath string) bool
	wrap   func(http.ResponseWriter, *http.Request, context.Context, NextAny) (any, error)
}

type NextAny func(context.Context) any

func New() *Router {
	return &Router{}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
