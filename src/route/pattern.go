package route

import (
	"fmt"
	"strings"
)

type Pattern struct {
	Method     string
	Path       string
	Segments   []string
	ParamNames []string
}

func Parse(endpoint string) (Pattern, error) {
	method, path, found := strings.Cut(strings.TrimSpace(endpoint), " ")
	if !found || method == "" || strings.TrimSpace(path) == "" {
		return Pattern{}, fmt.Errorf("endpoint path must look like 'METHOD /path', got %q", endpoint)
	}
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		return Pattern{}, fmt.Errorf("endpoint path must start with '/', got %q", path)
	}

	pattern := Pattern{Method: method, Path: path}
	if path == "/" {
		return pattern, nil
	}

	segments := strings.Split(strings.Trim(path, "/"), "/")
	pattern.Segments = segments
	for _, segment := range segments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
			if name == "" {
				return Pattern{}, fmt.Errorf("empty path parameter in %q", path)
			}
			pattern.ParamNames = append(pattern.ParamNames, name)
		}
	}

	return pattern, nil
}

func ExtractParams(pattern Pattern, requestPath string) (map[string]string, bool) {
	if pattern.Path == "/" && requestPath == "/" {
		return map[string]string{}, true
	}

	requestSegments := []string{}
	trimmed := strings.Trim(requestPath, "/")
	if trimmed != "" {
		requestSegments = strings.Split(trimmed, "/")
	}
	if len(requestSegments) != len(pattern.Segments) {
		return nil, false
	}

	params := map[string]string{}
	for i, segment := range pattern.Segments {
		requestSegment := requestSegments[i]
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
			params[name] = requestSegment
			continue
		}
		if segment != requestSegment {
			return nil, false
		}
	}

	return params, true
}
