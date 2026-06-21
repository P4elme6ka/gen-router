package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/P4elme6ka/gen-router/src/bind"
	"github.com/P4elme6ka/gen-router/src/render"
	"github.com/P4elme6ka/gen-router/src/route"
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

		finalNext := func(ctx context.Context) any {
			return handler.Handle(ctx, input)
		}
		next := r.composeMiddlewares(pattern.Method, req.URL.Path, w, req, finalNext)
		output, ok := next(req.Context()).(O)
		if !ok {
			http.Error(w, "failed to execute handler chain", http.StatusInternalServerError)
			return
		}
		if err := renderPlan.Write(w, output); err != nil {
			http.Error(w, fmt.Sprintf("failed to write output: %v", err), http.StatusInternalServerError)
		}
	}))

	return nil
}

func Use[I Input, O Output](r *Router, scope string, middleware Middleware[I, O]) error {
	scopePattern, err := parseMiddlewareScope(scope)
	if err != nil {
		return err
	}
	middlewarePattern, err := route.Parse(middleware.I().EndpointPath())
	if err != nil {
		return err
	}

	r.middlewares = append(r.middlewares, registeredMiddleware{
		method: scopePattern.Method,
		path:   scopePattern.Path,
		match:  prefixMatcher(scopePattern.Path),
		wrap: func(w http.ResponseWriter, req *http.Request, ctx context.Context, next NextAny) (any, error) {
			method := scopePattern.Method
			if method == "" {
				method = req.Method
			}
			requestPatternPath := alignPatternToRequest(middlewarePattern.Path, req.URL.Path)
			requestPattern, err := route.Parse(method + " " + requestPatternPath)
			if err != nil {
				return nil, err
			}
			bindPlan, err := bind.Compile(requestPattern, middleware.I())
			if err != nil {
				return nil, err
			}
			input, err := bindPlan.Parse(req)
			if err != nil {
				return nil, err
			}
			wrappedNext := func(nextCtx context.Context) O {
				result := next(nextCtx)
				output, ok := result.(O)
				if !ok {
					var zero O
					return zero
				}
				return output
			}
			return middleware.Handle(ctx, input, wrappedNext), nil
		},
	})
	return nil
}

func (r *Router) composeMiddlewares(method, requestPath string, w http.ResponseWriter, req *http.Request, finalNext NextAny) NextAny {
	next := finalNext
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		mw := r.middlewares[i]
		if mw.method != "" && mw.method != method {
			continue
		}
		if !mw.match(requestPath) {
			continue
		}
		current := mw
		previous := next
		next = func(ctx context.Context) any {
			result, err := current.wrap(w, req, ctx, previous)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid middleware input: %v", err), http.StatusBadRequest)
				return nil
			}
			return result
		}
	}
	return next
}

func prefixMatcher(scopePath string) func(string) bool {
	scopeSegments := splitPathSegments(scopePath)
	return func(requestPath string) bool {
		requestSegments := splitPathSegments(requestPath)
		if len(scopeSegments) > len(requestSegments) {
			return false
		}
		for i, scopeSegment := range scopeSegments {
			requestSegment := requestSegments[i]
			if strings.HasPrefix(scopeSegment, "{") && strings.HasSuffix(scopeSegment, "}") {
				continue
			}
			if scopeSegment != requestSegment {
				return false
			}
		}
		return true
	}
}

func splitPathSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func requestPatternForScope(scopePath, requestPath string) string {
	scopeSegments := splitPathSegments(scopePath)
	requestSegments := splitPathSegments(requestPath)
	if len(scopeSegments) == 0 {
		return requestPath
	}
	if len(scopeSegments) > len(requestSegments) {
		return scopePath
	}
	resolved := make([]string, 0, len(scopeSegments))
	for i, scopeSegment := range scopeSegments {
		if strings.HasPrefix(scopeSegment, "{") && strings.HasSuffix(scopeSegment, "}") {
			resolved = append(resolved, scopeSegment)
			continue
		}
		resolved = append(resolved, requestSegments[i])
	}
	return "/" + strings.Join(resolved, "/")
}

func alignPatternToRequest(patternPath, requestPath string) string {
	patternSegments := splitPathSegments(patternPath)
	requestSegments := splitPathSegments(requestPath)
	if len(patternSegments) == 0 || len(requestSegments) < len(patternSegments) {
		return patternPath
	}
	resolved := make([]string, 0, len(patternSegments))
	for i, patternSegment := range patternSegments {
		if strings.HasPrefix(patternSegment, "{") && strings.HasSuffix(patternSegment, "}") {
			resolved = append(resolved, patternSegment)
			continue
		}
		resolved = append(resolved, requestSegments[i])
	}
	return "/" + strings.Join(resolved, "/")
}

func parseMiddlewareScope(scope string) (route.Pattern, error) {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return route.Pattern{}, fmt.Errorf("middleware scope cannot be empty")
	}
	if strings.HasPrefix(scope, "/") {
		return route.Pattern{Path: scope}, nil
	}
	return route.Parse(scope)
}
