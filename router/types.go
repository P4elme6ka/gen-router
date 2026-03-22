package router

import "net/http"

type Input interface {
	EndpointPath() string
}

type Output interface{}

type Handler[I Input, O Output] interface {
	Handle(input I) O
	I() I
}

type Router struct {
	mux http.ServeMux
}

func New() *Router {
	return &Router{}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
