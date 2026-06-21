package oas

import (
	"fmt"
	"go/types"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/P4elme6ka/gen-router/src/codegen/ir"
)

type schemaBuilder struct {
	packagesByImportPath map[string]*packages.Package
	components           map[string]*Schema
	componentNames       map[string]string
	componentOwners      map[string]string
	building             map[string]bool
}

func newSchemaBuilder(plan *ir.ModulePlan) (*schemaBuilder, error) {
	importPaths := make([]string, 0, len(plan.Packages))
	seen := make(map[string]bool, len(plan.Packages))
	for _, pkg := range plan.Packages {
		if pkg.ImportPath == "" || seen[pkg.ImportPath] {
			continue
		}
		seen[pkg.ImportPath] = true
		importPaths = append(importPaths, pkg.ImportPath)
	}
	if len(importPaths) == 0 {
		return &schemaBuilder{
			packagesByImportPath: map[string]*packages.Package{},
			components:           map[string]*Schema{},
			componentNames:       map[string]string{},
			componentOwners:      map[string]string{},
			building:             map[string]bool{},
		}, nil
	}

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedImports | packages.NeedDeps,
	}, importPaths...)
	if err != nil {
		return nil, err
	}

	indexed := map[string]*packages.Package{}
	visited := map[string]bool{}
	for _, pkg := range pkgs {
		indexPackageTree(pkg, indexed, visited)
	}
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("failed to load package %s: %v", pkg.PkgPath, pkg.Errors[0])
		}
	}

	return &schemaBuilder{
		packagesByImportPath: indexed,
		components:           map[string]*Schema{},
		componentNames:       map[string]string{},
		componentOwners:      map[string]string{},
		building:             map[string]bool{},
	}, nil
}

func indexPackageTree(pkg *packages.Package, indexed map[string]*packages.Package, visited map[string]bool) {
	if pkg == nil || visited[pkg.PkgPath] {
		return
	}
	visited[pkg.PkgPath] = true
	indexed[pkg.PkgPath] = pkg
	for _, imported := range pkg.Imports {
		indexPackageTree(imported, indexed, visited)
	}
}

func (b *schemaBuilder) schemaForTypeString(currentImportPath, typeStr string) *Schema {
	t, err := b.resolveTypeString(currentImportPath, typeStr)
	if err != nil {
		return fallbackSchemaForTypeString(typeStr)
	}
	return b.schemaForType(t)
}

func (b *schemaBuilder) bodySchemaForTypeString(currentImportPath, typeStr string) *Schema {
	t, err := b.resolveTypeString(currentImportPath, typeStr)
	if err != nil {
		return fallbackSchemaForTypeString(typeStr)
	}
	return b.bodySchemaForType(t)
}

func (b *schemaBuilder) bodySchemaForType(t types.Type) *Schema {
	t = derefType(t)
	if named, ok := t.(*types.Named); ok {
		if _, ok := named.Underlying().(*types.Struct); ok {
			name := b.ensureComponent(named)
			return &Schema{Ref: "#/components/schemas/" + name}
		}
	}
	return b.schemaForType(t)
}

func (b *schemaBuilder) schemaForType(t types.Type) *Schema {
	t = derefType(t)
	switch current := t.(type) {
	case *types.Named:
		if schema, ok := specialNamedSchema(current); ok {
			return schema
		}
		if _, ok := current.Underlying().(*types.Struct); ok {
			name := b.ensureComponent(current)
			return &Schema{Ref: "#/components/schemas/" + name}
		}
		return b.schemaForType(current.Underlying())
	case *types.Basic:
		return basicSchema(current)
	case *types.Slice:
		if isByteType(current.Elem()) {
			return &Schema{Type: "string", Format: "byte"}
		}
		return &Schema{Type: "array", Items: b.schemaForType(current.Elem())}
	case *types.Array:
		if isByteType(current.Elem()) {
			return &Schema{Type: "string", Format: "byte"}
		}
		return &Schema{Type: "array", Items: b.schemaForType(current.Elem())}
	case *types.Map:
		if !isStringType(current.Key()) {
			return &Schema{Type: "object"}
		}
		return &Schema{Type: "object", AdditionalProperties: b.schemaForType(current.Elem())}
	case *types.Struct:
		return b.inlineStructSchema(current)
	case *types.Interface:
		return &Schema{}
	default:
		return &Schema{}
	}
}

func (b *schemaBuilder) inlineStructSchema(st *types.Struct) *Schema {
	schema := &Schema{Type: "object", Properties: map[string]*Schema{}}
	required := make([]string, 0)

	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		if !field.Exported() {
			continue
		}

		jsonName, omitEmpty, skip := parseJSONField(st.Tag(i), field.Name())
		if skip {
			continue
		}

		if field.Anonymous() && jsonName == field.Name() {
			if embeddedSchema, ok := b.flattenAnonymousField(field.Type()); ok {
				for name, prop := range embeddedSchema.Properties {
					schema.Properties[name] = prop
				}
				required = append(required, embeddedSchema.Required...)
				continue
			}
		}

		schema.Properties[jsonName] = b.schemaForType(field.Type())
		if !omitEmpty {
			required = append(required, jsonName)
		}
	}

	if len(required) > 0 {
		sort.Strings(required)
		schema.Required = uniqueStrings(required)
	}
	if len(schema.Properties) == 0 {
		schema.Properties = nil
	}
	return schema
}

func (b *schemaBuilder) flattenAnonymousField(t types.Type) (*Schema, bool) {
	t = derefType(t)
	named, ok := t.(*types.Named)
	if ok {
		t = named.Underlying()
	}
	st, ok := t.(*types.Struct)
	if !ok {
		return nil, false
	}
	return b.inlineStructSchema(st), true
}

func (b *schemaBuilder) ensureComponent(named *types.Named) string {
	key := namedTypeKey(named)
	if name, ok := b.componentNames[key]; ok {
		if b.building[key] {
			return name
		}
		if _, exists := b.components[name]; exists {
			return name
		}
	}

	name := b.allocateComponentName(named)
	b.componentNames[key] = name
	if b.building[key] {
		return name
	}

	b.building[key] = true
	b.components[name] = b.inlineStructSchema(named.Underlying().(*types.Struct))
	delete(b.building, key)
	return name
}

func (b *schemaBuilder) allocateComponentName(named *types.Named) string {
	key := namedTypeKey(named)
	if existing, ok := b.componentNames[key]; ok {
		return existing
	}

	base := named.Obj().Name()
	candidate := base
	if owner, exists := b.componentOwners[candidate]; exists && owner != key {
		candidate = named.Obj().Pkg().Name() + "." + base
	}
	if owner, exists := b.componentOwners[candidate]; exists && owner != key {
		candidate = sanitizeComponentName(named.Obj().Pkg().Path()) + "." + base
	}

	b.componentOwners[candidate] = key
	return candidate
}

func namedTypeKey(named *types.Named) string {
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return named.String()
	}
	return named.Obj().Pkg().Path() + "." + named.Obj().Name()
}

func sanitizeComponentName(value string) string {
	replacer := strings.NewReplacer("/", ".", "-", "_", " ", "_")
	return replacer.Replace(value)
}

func (b *schemaBuilder) resolveTypeString(currentImportPath, typeStr string) (types.Type, error) {
	typeStr = strings.TrimSpace(typeStr)
	switch {
	case strings.HasPrefix(typeStr, "*"):
		elem, err := b.resolveTypeString(currentImportPath, strings.TrimPrefix(typeStr, "*"))
		if err != nil {
			return nil, err
		}
		return types.NewPointer(elem), nil
	case strings.HasPrefix(typeStr, "[]"):
		elem, err := b.resolveTypeString(currentImportPath, strings.TrimPrefix(typeStr, "[]"))
		if err != nil {
			return nil, err
		}
		return types.NewSlice(elem), nil
	case strings.HasPrefix(typeStr, "map[string]"):
		elem, err := b.resolveTypeString(currentImportPath, strings.TrimPrefix(typeStr, "map[string]"))
		if err != nil {
			return nil, err
		}
		return types.NewMap(types.Typ[types.String], elem), nil
	case typeStr == "any" || typeStr == "interface{}":
		return types.NewInterfaceType(nil, nil).Complete(), nil
	}

	if builtin := types.Universe.Lookup(typeStr); builtin != nil && builtin.Type() != nil {
		return builtin.Type(), nil
	}

	pkg := b.packagesByImportPath[currentImportPath]
	if pkg == nil || pkg.Types == nil {
		return nil, fmt.Errorf("package %s not loaded", currentImportPath)
	}

	if qualifier, name, found := strings.Cut(typeStr, "."); found {
		imported := findImportedPackageByName(pkg, qualifier)
		if imported == nil || imported.Types == nil {
			return nil, fmt.Errorf("package qualifier %s not found in %s", qualifier, currentImportPath)
		}
		obj := imported.Types.Scope().Lookup(name)
		if obj == nil {
			return nil, fmt.Errorf("type %s not found in package %s", name, imported.PkgPath)
		}
		return obj.Type(), nil
	}

	obj := pkg.Types.Scope().Lookup(typeStr)
	if obj == nil {
		return nil, fmt.Errorf("type %s not found in package %s", typeStr, currentImportPath)
	}
	return obj.Type(), nil
}

func findImportedPackageByName(pkg *packages.Package, name string) *packages.Package {
	for _, imported := range pkg.Imports {
		if imported != nil && imported.Name == name {
			return imported
		}
	}
	return nil
}

func parseJSONField(rawTag, defaultName string) (name string, omitEmpty bool, skip bool) {
	name = defaultName
	tag := reflect.StructTag(rawTag).Get("json")
	if tag == "" {
		return name, false, false
	}
	parts := strings.Split(tag, ",")
	if len(parts) > 0 {
		switch parts[0] {
		case "-":
			return "", false, true
		case "":
		default:
			name = parts[0]
		}
	}
	for _, part := range parts[1:] {
		if part == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, false
}

func derefType(t types.Type) types.Type {
	for {
		ptr, ok := t.(*types.Pointer)
		if !ok {
			return t
		}
		t = ptr.Elem()
	}
}

func basicSchema(basic *types.Basic) *Schema {
	info := basic.Info()
	switch {
	case info&types.IsBoolean != 0:
		return &Schema{Type: "boolean"}
	case info&types.IsInteger != 0:
		switch basic.Kind() {
		case types.Int64:
			return &Schema{Type: "integer", Format: "int64"}
		case types.Uint64:
			return &Schema{Type: "integer", Format: "uint64"}
		case types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uintptr:
			return &Schema{Type: "integer", Format: "uint32"}
		default:
			return &Schema{Type: "integer", Format: "int32"}
		}
	case info&types.IsFloat != 0:
		if basic.Kind() == types.Float32 {
			return &Schema{Type: "number", Format: "float"}
		}
		return &Schema{Type: "number", Format: "double"}
	case info&types.IsString != 0:
		return &Schema{Type: "string"}
	default:
		return &Schema{}
	}
}

func specialNamedSchema(named *types.Named) (*Schema, bool) {
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return nil, false
	}
	switch named.Obj().Pkg().Path() + "." + named.Obj().Name() {
	case "github.com/google/uuid.UUID":
		return &Schema{Type: "string", Format: "uuid"}, true
	case "time.Time":
		return &Schema{Type: "string", Format: "date-time"}, true
	}
	return nil, false
}

func isStringType(t types.Type) bool {
	basic, ok := derefType(t).(*types.Basic)
	return ok && basic.Kind() == types.String
}

func isByteType(t types.Type) bool {
	basic, ok := derefType(t).(*types.Basic)
	return ok && (basic.Kind() == types.Byte || basic.Kind() == types.Uint8)
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := values[:0]
	var prev string
	for i, value := range values {
		if i == 0 || value != prev {
			result = append(result, value)
			prev = value
		}
	}
	return result
}

func fallbackSchemaForTypeString(typeName string) *Schema {
	typeName = strings.TrimPrefix(typeName, "*")
	if strings.HasPrefix(typeName, "[]") {
		return &Schema{Type: "array", Items: fallbackSchemaForTypeString(strings.TrimPrefix(typeName, "[]"))}
	}
	switch typeName {
	case "string":
		return &Schema{Type: "string"}
	case "bool":
		return &Schema{Type: "boolean"}
	case "int", "int8", "int16", "int32":
		return &Schema{Type: "integer", Format: "int32"}
	case "int64":
		return &Schema{Type: "integer", Format: "int64"}
	case "uint", "uint8", "uint16", "uint32", "uintptr", "byte":
		return &Schema{Type: "integer", Format: "uint32"}
	case "uint64":
		return &Schema{Type: "integer", Format: "uint64"}
	case "float32":
		return &Schema{Type: "number", Format: "float"}
	case "float64":
		return &Schema{Type: "number", Format: "double"}
	case "uuid.UUID", "github.com/google/uuid.UUID":
		return &Schema{Type: "string", Format: "uuid"}
	default:
		return &Schema{Ref: "#/components/schemas/" + extractTypeName(typeName)}
	}
}
