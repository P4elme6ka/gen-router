package render

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"gen-router/internal/meta"
)

type accessor func(reflect.Value) reflect.Value

type Plan[O any] struct {
	rootType     reflect.Type
	initValue    func(O) (reflect.Value, error)
	sharedFields []fieldPlan
	variants     []variantPlan
}

type variantPlan struct {
	statusCode int
	fields     []fieldPlan
}

type fieldPlan struct {
	source string
	name   string
	access accessor
	isSet  func(reflect.Value) bool
	write  func(http.ResponseWriter, reflect.Value) error
}

func Compile[O any](sample O) (*Plan[O], error) {
	var zero O
	rootType := reflect.TypeOf(sample)
	if rootType == nil {
		rootType = reflect.TypeOf(zero)
	}
	if rootType == nil {
		return nil, fmt.Errorf("output type is nil")
	}

	storageType := rootType
	if storageType.Kind() == reflect.Pointer {
		storageType = storageType.Elem()
	}
	if storageType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("output type %s must be a struct or pointer to struct", rootType)
	}

	descriptor, err := meta.DescribeOutput(storageType)
	if err != nil {
		return nil, err
	}

	plan := &Plan[O]{
		rootType: storageType,
		initValue: func(output O) (reflect.Value, error) {
			value := reflect.ValueOf(output)
			if !value.IsValid() {
				return reflect.Value{}, fmt.Errorf("output is invalid")
			}
			if value.Kind() == reflect.Pointer {
				if value.IsNil() {
					return reflect.Value{}, fmt.Errorf("output is nil")
				}
				value = value.Elem()
			}
			return value, nil
		},
	}

	for _, field := range descriptor.SharedFields {
		compiledField, err := compileField(field)
		if err != nil {
			return nil, err
		}
		plan.sharedFields = append(plan.sharedFields, compiledField)
	}

	for _, variant := range descriptor.Variants {
		compiledVariant := variantPlan{statusCode: variant.StatusCode}
		for _, field := range variant.Fields {
			compiledField, err := compileField(field)
			if err != nil {
				return nil, err
			}
			compiledVariant.fields = append(compiledVariant.fields, compiledField)
		}
		plan.variants = append(plan.variants, compiledVariant)
	}

	return plan, nil
}

func WriteOutput[O any](w http.ResponseWriter, output O) error {
	plan, err := Compile(output)
	if err != nil {
		return err
	}
	return plan.Write(w, output)
}

func (p *Plan[O]) Write(w http.ResponseWriter, output O) error {
	value, err := p.initValue(output)
	if err != nil {
		return err
	}

	variant, ok := p.selectVariant(value)
	if !ok {
		return fmt.Errorf("no suitable response variant found for %s", p.rootType)
	}

	bodyWritten := false
	for _, field := range p.sharedFields {
		fieldValue := field.access(value)
		if !field.isSet(fieldValue) {
			continue
		}
		if field.source == "body" {
			return fmt.Errorf("shared output field %q cannot use body source without response status", field.name)
		}
		if err := field.write(w, fieldValue); err != nil {
			return err
		}
	}

	for _, field := range variant.fields {
		fieldValue := field.access(value)
		if !field.isSet(fieldValue) {
			continue
		}
		if field.source == "body" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(variant.statusCode)
			if err := field.write(w, fieldValue); err != nil {
				return err
			}
			bodyWritten = true
			continue
		}
		if err := field.write(w, fieldValue); err != nil {
			return err
		}
	}

	if bodyWritten {
		return nil
	}

	w.WriteHeader(variant.statusCode)
	return nil
}

func (p *Plan[O]) selectVariant(value reflect.Value) (*variantPlan, bool) {
	for i := range p.variants {
		variant := &p.variants[i]
		for _, field := range variant.fields {
			if field.isSet(field.access(value)) {
				return variant, true
			}
		}
	}
	return nil, false
}

func compileField(field meta.OutputField) (fieldPlan, error) {
	plan := fieldPlan{
		source: field.Source,
		name:   field.Name,
		access: compileAccessor(field.FieldIndex),
		isSet:  isPresent,
	}

	switch field.Source {
	case "header":
		plan.write = func(w http.ResponseWriter, value reflect.Value) error {
			w.Header().Set(field.Name, fmt.Sprint(value.Interface()))
			return nil
		}
	case "body":
		plan.write = func(w http.ResponseWriter, value reflect.Value) error {
			return json.NewEncoder(w).Encode(value.Interface())
		}
	default:
		return fieldPlan{}, fmt.Errorf("unsupported output source %q", field.Source)
	}

	return plan, nil
}

func isPresent(value reflect.Value) bool {
	if value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		return !value.IsNil()
	}
	return !value.IsZero()
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
