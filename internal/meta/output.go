package meta

import (
	"fmt"
	"reflect"
	"sync"
)

type OutputField struct {
	FieldIndex []int
	GoName     string
	Type       reflect.Type
	Source     string
	Name       string
}

type OutputVariant struct {
	StatusCode int
	Fields     []OutputField
}

type OutputMetadata struct {
	Type         reflect.Type
	SharedFields []OutputField
	Variants     []OutputVariant
}

var outputCache sync.Map

func DescribeOutput(t reflect.Type) (*OutputMetadata, error) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if cached, ok := outputCache.Load(t); ok {
		return cached.(*OutputMetadata), nil
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("output type %s must be a struct", t)
	}

	meta := &OutputMetadata{Type: t}
	variantByCode := map[int]*OutputVariant{}
	orderedCodes := []int{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		tag, err := ParseTag(field.Tag.Get("gen-router"))
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}

		source := tag.Source()
		status, hasResponse, err := tag.ResponseCode()
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}
		if source == "" && !hasResponse {
			continue
		}
		if source == "" {
			source = "body"
		}
		if source != "body" && source != "header" {
			return nil, fmt.Errorf("field %s: unsupported output source %q", field.Name, source)
		}
		name := tag.Name()
		if name == "" {
			name = field.Name
		}

		outputField := OutputField{
			FieldIndex: field.Index,
			GoName:     field.Name,
			Type:       field.Type,
			Source:     source,
			Name:       name,
		}

		if !hasResponse {
			meta.SharedFields = append(meta.SharedFields, outputField)
			continue
		}

		variant := variantByCode[status]
		if variant == nil {
			variant = &OutputVariant{StatusCode: status}
			variantByCode[status] = variant
			orderedCodes = append(orderedCodes, status)
		}
		variant.Fields = append(variant.Fields, outputField)
	}

	for _, code := range orderedCodes {
		meta.Variants = append(meta.Variants, *variantByCode[code])
	}
	if len(meta.Variants) == 0 {
		return nil, fmt.Errorf("output type %s has no response variants", t)
	}

	outputCache.Store(t, meta)
	return meta, nil
}
