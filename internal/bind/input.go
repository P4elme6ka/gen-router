package bind

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	"github.com/google/uuid"

	"github.com/P4elme6ka/gen-router/internal/meta"
	"github.com/P4elme6ka/gen-router/internal/route"
)

type Plan[I any] struct {
	pattern     route.Pattern
	rootType    reflect.Type
	initRoot    func() reflect.Value
	finish      func(reflect.Value) I
	bodyStep    *bodyStep
	pathSteps   []valueStep
	querySteps  []valueStep
	headerSteps []valueStep
	cookieSteps []cookieStep
}

type accessor func(reflect.Value) reflect.Value

type bodyStep struct {
	field  meta.InputField
	access accessor
}

type valueStep struct {
	field        meta.InputField
	access       accessor
	required     bool
	slice        bool
	lookupName   string
	assignSingle func(reflect.Value, string) error
	assignMany   func(reflect.Value, []string) error
}

type cookieStep struct {
	field      meta.InputField
	access     accessor
	required   bool
	lookupName string
	assign     func(reflect.Value, string) error
}

func Compile[I any](pattern route.Pattern, sample I) (*Plan[I], error) {
	var zero I
	rootType := reflect.TypeOf(sample)
	if rootType == nil {
		rootType = reflect.TypeOf(zero)
	}
	if rootType == nil {
		return nil, fmt.Errorf("input type is nil")
	}

	storageType := rootType
	if storageType.Kind() == reflect.Pointer {
		storageType = storageType.Elem()
	}
	if storageType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("input type %s must be a struct or pointer to struct", rootType)
	}

	descriptor, err := meta.DescribeInput(storageType)
	if err != nil {
		return nil, err
	}

	plan := &Plan[I]{
		pattern:  pattern,
		rootType: storageType,
		initRoot: func() reflect.Value {
			return reflect.New(storageType).Elem()
		},
		finish: func(root reflect.Value) I {
			if rootType.Kind() == reflect.Pointer {
				ptr := reflect.New(storageType)
				ptr.Elem().Set(root)
				return ptr.Interface().(I)
			}
			return root.Interface().(I)
		},
	}

	if descriptor.BodyField != nil {
		plan.bodyStep = &bodyStep{field: *descriptor.BodyField, access: compileAccessor(descriptor.BodyField.FieldIndex)}
	}
	if plan.pathSteps, err = compileValueSteps(descriptor.PathFields); err != nil {
		return nil, err
	}
	if plan.querySteps, err = compileValueSteps(descriptor.QueryFields); err != nil {
		return nil, err
	}
	if plan.headerSteps, err = compileValueSteps(descriptor.HeaderFields); err != nil {
		return nil, err
	}
	if plan.cookieSteps, err = compileCookieSteps(descriptor.CookieFields); err != nil {
		return nil, err
	}

	return plan, nil
}

func ParseInput[I any](req *http.Request, pattern route.Pattern, sample I) (I, error) {
	plan, err := Compile(pattern, sample)
	if err != nil {
		var zero I
		return zero, err
	}
	return plan.Parse(req)
}

func (p *Plan[I]) Parse(req *http.Request) (I, error) {
	root := p.initRoot()
	if p.bodyStep != nil {
		if err := p.bodyStep.apply(req, root); err != nil {
			var zero I
			return zero, err
		}
	}

	pathParams, ok := route.ExtractParams(p.pattern, req.URL.Path)
	if !ok {
		var zero I
		return zero, fmt.Errorf("request path %q does not match pattern %q", req.URL.Path, p.pattern.Path)
	}
	for _, step := range p.pathSteps {
		raw, exists := pathParams[step.lookupName]
		if err := step.applyRaw(root, raw, exists); err != nil {
			var zero I
			return zero, err
		}
	}

	query := req.URL.Query()
	for _, step := range p.querySteps {
		values, exists := query[step.lookupName]
		if err := step.applyValues(root, values, exists); err != nil {
			var zero I
			return zero, err
		}
	}

	for _, step := range p.headerSteps {
		values, exists := req.Header[http.CanonicalHeaderKey(step.lookupName)]
		if err := step.applyValues(root, values, exists); err != nil {
			var zero I
			return zero, err
		}
	}

	for _, step := range p.cookieSteps {
		cookie, err := req.Cookie(step.lookupName)
		if err != nil {
			if err == http.ErrNoCookie {
				if !step.required {
					continue
				}
				var zero I
				return zero, fmt.Errorf("missing cookie %q", step.lookupName)
			}
			var zero I
			return zero, err
		}
		if err := step.assign(step.access(root), cookie.Value); err != nil {
			var zero I
			return zero, fmt.Errorf("parse %s parameter %q: %w", step.field.Source, step.field.Name, err)
		}
	}

	return p.finish(root), nil
}

func compileValueSteps(fields []meta.InputField) ([]valueStep, error) {
	steps := make([]valueStep, 0, len(fields))
	for _, field := range fields {
		assignSingle, assignMany, err := compileAssignments(field)
		if err != nil {
			return nil, err
		}
		steps = append(steps, valueStep{
			field:        field,
			access:       compileAccessor(field.FieldIndex),
			required:     field.Type.Kind() != reflect.Pointer,
			slice:        derefType(field.Type).Kind() == reflect.Slice,
			lookupName:   field.Name,
			assignSingle: assignSingle,
			assignMany:   assignMany,
		})
	}
	return steps, nil
}

func compileCookieSteps(fields []meta.InputField) ([]cookieStep, error) {
	steps := make([]cookieStep, 0, len(fields))
	for _, field := range fields {
		assignSingle, _, err := compileAssignments(field)
		if err != nil {
			return nil, err
		}
		steps = append(steps, cookieStep{
			field:      field,
			access:     compileAccessor(field.FieldIndex),
			required:   field.Type.Kind() != reflect.Pointer,
			lookupName: field.Name,
			assign:     assignSingle,
		})
	}
	return steps, nil
}

func compileAssignments(field meta.InputField) (func(reflect.Value, string) error, func(reflect.Value, []string) error, error) {
	targetType := field.Type
	if targetType.Kind() == reflect.Pointer {
		assignElem, err := compileDirectScalarSetter(targetType.Elem())
		if err != nil {
			return nil, nil, fmt.Errorf("field %s: %w", field.GoName, err)
		}
		return func(target reflect.Value, raw string) error {
			ptr := reflect.New(targetType.Elem())
			if err := assignElem(ptr.Elem(), raw); err != nil {
				return err
			}
			target.Set(ptr)
			return nil
		}, nil, nil
	}

	if targetType.Kind() == reflect.Slice {
		elementSetter, err := compileDirectScalarSetter(targetType.Elem())
		if err != nil {
			return nil, nil, fmt.Errorf("field %s: %w", field.GoName, err)
		}
		return nil, func(target reflect.Value, raws []string) error {
			result := reflect.MakeSlice(targetType, 0, len(raws))
			for _, raw := range raws {
				element := reflect.New(targetType.Elem()).Elem()
				if err := elementSetter(element, raw); err != nil {
					return err
				}
				result = reflect.Append(result, element)
			}
			target.Set(result)
			return nil
		}, nil
	}

	assignScalar, err := compileDirectScalarSetter(targetType)
	if err != nil {
		return nil, nil, fmt.Errorf("field %s: %w", field.GoName, err)
	}
	return func(target reflect.Value, raw string) error {
		return assignScalar(target, raw)
	}, nil, nil
}

func compileDirectScalarSetter(targetType reflect.Type) (func(reflect.Value, string) error, error) {
	if targetType == reflect.TypeOf(uuid.UUID{}) {
		return func(target reflect.Value, raw string) error {
			parsed, err := uuid.Parse(raw)
			if err != nil {
				return err
			}
			target.Set(reflect.ValueOf(parsed))
			return nil
		}, nil
	}

	switch targetType.Kind() {
	case reflect.String:
		return func(target reflect.Value, raw string) error {
			target.SetString(raw)
			return nil
		}, nil
	case reflect.Bool:
		return func(target reflect.Value, raw string) error {
			parsed, err := strconv.ParseBool(raw)
			if err != nil {
				return err
			}
			target.SetBool(parsed)
			return nil
		}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return func(target reflect.Value, raw string) error {
			parsed, err := strconv.ParseInt(raw, 10, targetType.Bits())
			if err != nil {
				return err
			}
			target.SetInt(parsed)
			return nil
		}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return func(target reflect.Value, raw string) error {
			parsed, err := strconv.ParseUint(raw, 10, targetType.Bits())
			if err != nil {
				return err
			}
			target.SetUint(parsed)
			return nil
		}, nil
	case reflect.Float32, reflect.Float64:
		return func(target reflect.Value, raw string) error {
			parsed, err := strconv.ParseFloat(raw, targetType.Bits())
			if err != nil {
				return err
			}
			target.SetFloat(parsed)
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("unsupported target type %s", targetType)
	}
}

func (s valueStep) applyValues(root reflect.Value, values []string, exists bool) error {
	target := s.access(root)
	if s.slice {
		if !exists {
			if !s.required {
				return nil
			}
			return fmt.Errorf("missing %s parameter %q", s.field.Source, s.field.Name)
		}
		return wrapAssignError(s.field, s.assignMany(target, values))
	}
	if !exists || len(values) == 0 {
		if !s.required {
			return nil
		}
		return fmt.Errorf("missing %s parameter %q", s.field.Source, s.field.Name)
	}
	return s.applyRaw(root, values[0], true)
}

func (s valueStep) applyRaw(root reflect.Value, raw string, exists bool) error {
	if !exists {
		if !s.required {
			return nil
		}
		return fmt.Errorf("missing %s parameter %q", s.field.Source, s.field.Name)
	}
	return wrapAssignError(s.field, s.assignSingle(s.access(root), raw))
}

func wrapAssignError(field meta.InputField, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("parse %s parameter %q: %w", field.Source, field.Name, err)
}

func (s bodyStep) apply(req *http.Request, root reflect.Value) error {
	bodyValue := s.access(root)
	if !reqBodyPresent(req) {
		if bodyValue.Kind() == reflect.Pointer {
			return nil
		}
		return fmt.Errorf("missing request body")
	}

	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if bodyValue.Kind() == reflect.Pointer {
		bodyValue.Set(reflect.New(bodyValue.Type().Elem()))
		if err := decoder.Decode(bodyValue.Interface()); err != nil {
			return fmt.Errorf("decode body into %s: %w", s.field.GoName, err)
		}
		return nil
	}
	if err := decoder.Decode(bodyValue.Addr().Interface()); err != nil {
		return fmt.Errorf("decode body into %s: %w", s.field.GoName, err)
	}
	return nil
}

func reqBodyPresent(req *http.Request) bool {
	if req.Body == nil {
		return false
	}
	if req.ContentLength > 0 {
		return true
	}
	if req.ContentLength == 0 {
		return false
	}
	return true
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func compileAccessor(index []int) accessor {
	copied := append([]int(nil), index...)
	return func(root reflect.Value) reflect.Value {
		current := root
		for _, idx := range copied {
			current = current.Field(idx)
		}
		return current
	}
}
