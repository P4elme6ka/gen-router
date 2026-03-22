package router

import (
	"fmt"
	"net/http"

	"gen-router/internal/bind"
	"gen-router/internal/render"
	"gen-router/internal/route"
)

func Register[I Input, O Output](r *Router, handler Handler[I, O]) error {
	pattern, err := route.Parse(handler.I().EndpointPath())
	if err != nil {
		return err
	}

	bindPlan, err := bind.Compile(pattern, handler.I())
	if err != nil {
		return err
	}

	var outputSample O
	renderPlan, err := render.Compile(outputSample)
	if err != nil {
		return err
	}

	r.mux.Handle(pattern.Method+" "+pattern.Path, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		input, err := bindPlan.Parse(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid input: %v", err), http.StatusBadRequest)
			return
		}
		output := handler.Handle(input)
		if err := renderPlan.Write(w, output); err != nil {
			http.Error(w, fmt.Sprintf("failed to write output: %v", err), http.StatusInternalServerError)
		}
	}))

	return nil
}
