package meta

import (
	"fmt"
	"reflect"
	"sync"
)

type InputField struct {
	FieldIndex []int
	GoName     string
	Source     string
	Name       string
	Type       reflect.Type
}

type InputMetadata struct {
	Type         reflect.Type
	BodyField    *InputField
	PathFields   []InputField
	QueryFields  []InputField
	HeaderFields []InputField
	CookieFields []InputField
}

var inputCache sync.Map

func DescribeInput(t reflect.Type) (*InputMetadata, error) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if cached, ok := inputCache.Load(t); ok {
		return cached.(*InputMetadata), nil
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("input type %s must be a struct", t)
	}

	meta := &InputMetadata{Type: t}
	seen := map[string]struct{}{}
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
		if source == "" {
			continue
		}

		descriptor := InputField{
			FieldIndex: field.Index,
			GoName:     field.Name,
			Source:     source,
			Name:       tag.Name(),
			Type:       field.Type,
		}
		if descriptor.Name == "" {
			descriptor.Name = field.Name
		}

		switch source {
		case "body":
			if meta.BodyField != nil {
				return nil, fmt.Errorf("multiple body fields are not supported")
			}
			bodyField := descriptor
			meta.BodyField = &bodyField
		case "path":
			if err := addInputField(seen, &meta.PathFields, descriptor); err != nil {
				return nil, err
			}
		case "query":
			if err := addInputField(seen, &meta.QueryFields, descriptor); err != nil {
				return nil, err
			}
		case "header":
			if err := addInputField(seen, &meta.HeaderFields, descriptor); err != nil {
				return nil, err
			}
		case "cookie":
			if err := addInputField(seen, &meta.CookieFields, descriptor); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("field %s: unsupported input source %q", field.Name, source)
		}
	}

	inputCache.Store(t, meta)
	return meta, nil
}

func addInputField(seen map[string]struct{}, target *[]InputField, descriptor InputField) error {
	key := descriptor.Source + ":" + descriptor.Name
	if _, exists := seen[key]; exists {
		return fmt.Errorf("duplicate %s binding for %q", descriptor.Source, descriptor.Name)
	}
	seen[key] = struct{}{}
	*target = append(*target, descriptor)
	return nil
}
