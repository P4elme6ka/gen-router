package discover

import (
	"context"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/P4elme6ka/gen-router/internal/codegen/ir"
)

type Config struct {
	Patterns []string
}

const routerPackagePath = "github.com/P4elme6ka/gen-router/router"

func LoadModulePlan(cfg Config) (*ir.ModulePlan, error) {
	if len(cfg.Patterns) == 0 {
		cfg.Patterns = []string{"./..."}
	}

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
	}, cfg.Patterns...)
	if err != nil {
		return nil, err
	}

	modulePlan := &ir.ModulePlan{}
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("failed to load package %s: %v", pkg.PkgPath, pkg.Errors[0])
		}
		packagePlan, err := buildPackagePlan(pkg)
		if err != nil {
			return nil, err
		}
		modulePlan.Packages = append(modulePlan.Packages, packagePlan)
	}

	sort.Slice(modulePlan.Packages, func(i, j int) bool {
		return modulePlan.Packages[i].ImportPath < modulePlan.Packages[j].ImportPath
	})
	return modulePlan, nil
}

func buildPackagePlan(pkg *packages.Package) (ir.PackagePlan, error) {
	plan := ir.PackagePlan{ImportPath: pkg.PkgPath, PackageName: pkg.Name}
	if len(pkg.GoFiles) > 0 {
		plan.Dir = filepath.Dir(pkg.GoFiles[0])
	} else if len(pkg.CompiledGoFiles) > 0 {
		plan.Dir = filepath.Dir(pkg.CompiledGoFiles[0])
	}

	scope := pkg.Types.Scope()
	names := scope.Names()
	sort.Strings(names)
	seenHandlers := map[string]struct{}{}

	for _, name := range names {
		obj := scope.Lookup(name)
		typeName, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		named, ok := typeName.Type().(*types.Named)
		if !ok {
			continue
		}
		handlerPlan, ok, err := buildHandlerPlan(pkg, named)
		if err != nil {
			return ir.PackagePlan{}, err
		}
		if !ok {
			continue
		}
		if _, exists := seenHandlers[handlerPlan.HandlerType]; exists {
			continue
		}
		seenHandlers[handlerPlan.HandlerType] = struct{}{}
		plan.Handlers = append(plan.Handlers, handlerPlan)
	}

	return plan, nil
}

func buildHandlerPlan(pkg *packages.Package, handlerType *types.Named) (ir.HandlerPlan, bool, error) {
	if handlerType.Obj() == nil || handlerType.Obj().Pkg() == nil || handlerType.Obj().Pkg().Path() != pkg.PkgPath {
		return ir.HandlerPlan{}, false, nil
	}
	if _, isInterface := handlerType.Underlying().(*types.Interface); isInterface {
		return ir.HandlerPlan{}, false, nil
	}

	inputType, outputType, ok := discoverHandlerTypes(handlerType)
	if !ok {
		return ir.HandlerPlan{}, false, nil
	}
	if namedType(inputType) == nil || namedType(outputType) == nil {
		return ir.HandlerPlan{}, false, nil
	}

	endpoint, err := discoverEndpointPath(pkg, handlerType)
	if err != nil {
		return ir.HandlerPlan{}, false, fmt.Errorf("handler %s: %w", handlerType.Obj().Name(), err)
	}
	method, path, err := splitEndpoint(endpoint)
	if err != nil {
		return ir.HandlerPlan{}, false, fmt.Errorf("handler %s: %w", handlerType.Obj().Name(), err)
	}

	inputPlan, err := buildInputPlan(inputType)
	if err != nil {
		return ir.HandlerPlan{}, false, fmt.Errorf("handler %s input %s: %w", handlerType.Obj().Name(), types.TypeString(inputType, packageQualifier(pkg)), err)
	}
	outputPlan, err := buildOutputPlan(outputType)
	if err != nil {
		return ir.HandlerPlan{}, false, fmt.Errorf("handler %s output %s: %w", handlerType.Obj().Name(), types.TypeString(outputType, packageQualifier(pkg)), err)
	}

	return ir.HandlerPlan{
		HandlerType: types.TypeString(handlerType, packageQualifier(pkg)),
		InputType:   types.TypeString(inputType, packageQualifier(pkg)),
		OutputType:  types.TypeString(outputType, packageQualifier(pkg)),
		Route: ir.RoutePlan{
			Method: method,
			Path:   path,
		},
		Input:  inputPlan,
		Output: outputPlan,
	}, true, nil
}

func discoverHandlerTypes(handlerType *types.Named) (types.Type, types.Type, bool) {
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
	for _, candidate := range []types.Type{handlerType, types.NewPointer(handlerType)} {
		methodSet := types.NewMethodSet(candidate)
		handleMethod := methodSet.Lookup(nil, "Handle")
		iMethod := methodSet.Lookup(nil, "I")
		if handleMethod == nil || iMethod == nil {
			continue
		}

		handleSig, ok := handleMethod.Obj().Type().(*types.Signature)
		if !ok || handleSig.Results().Len() != 1 {
			continue
		}
		if handleSig.Params().Len() != 2 {
			continue
		}
		if types.TypeString(handleSig.Params().At(0).Type(), nil) != contextType.String() {
			continue
		}
		iSig, ok := iMethod.Obj().Type().(*types.Signature)
		if !ok || iSig.Params().Len() != 0 || iSig.Results().Len() != 1 {
			continue
		}

		inputType := handleSig.Params().At(1).Type()
		if !types.Identical(inputType, iSig.Results().At(0).Type()) {
			continue
		}
		return inputType, handleSig.Results().At(0).Type(), true
	}

	return nil, nil, false
}

func discoverEndpointPath(pkg *packages.Package, handlerType *types.Named) (string, error) {
	inputType, _, ok := discoverHandlerTypes(handlerType)
	if !ok {
		return "", fmt.Errorf("could not infer handler input type")
	}
	inputNamed := namedType(inputType)
	if inputNamed == nil {
		return "", fmt.Errorf("input type %s is not a named type", types.TypeString(inputType, packageQualifier(pkg)))
	}

	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name == nil || fn.Name.Name != "EndpointPath" {
				continue
			}
			if len(fn.Recv.List) != 1 {
				continue
			}
			recvName := receiverTypeName(fn.Recv.List[0].Type)
			if recvName != inputNamed.Obj().Name() {
				continue
			}
			if len(fn.Body.List) != 1 {
				continue
			}
			ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				continue
			}
			value, err := evaluateStringExpr(pkg, file, ret.Results[0])
			if err != nil {
				return "", fmt.Errorf("EndpointPath expression is not supported for codegen: %w", err)
			}
			return value, nil
		}
	}

	return "", fmt.Errorf("could not find compile-time EndpointPath implementation for %s", inputNamed.Obj().Name())
}

func evaluateStringExpr(pkg *packages.Package, file *ast.File, expr ast.Expr) (string, error) {
	switch value := expr.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", fmt.Errorf("expected string literal")
		}
		return strconv.Unquote(value.Value)
	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return "", fmt.Errorf("unsupported binary operator %s", value.Op)
		}
		left, err := evaluateStringExpr(pkg, file, value.X)
		if err != nil {
			return "", err
		}
		right, err := evaluateStringExpr(pkg, file, value.Y)
		if err != nil {
			return "", err
		}
		return left + right, nil
	case *ast.CallExpr:
		return evaluateStringCallExpr(pkg, file, value)
	case *ast.Ident:
		return evaluateIdentString(pkg, file, value)
	case *ast.SelectorExpr:
		return evaluateSelectorString(pkg, file, value)
	case *ast.ParenExpr:
		return evaluateStringExpr(pkg, file, value.X)
	default:
		return "", fmt.Errorf("unsupported expression type %T", expr)
	}
}

func evaluateStringCallExpr(pkg *packages.Package, file *ast.File, call *ast.CallExpr) (string, error) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", fmt.Errorf("unsupported call expression")
	}
	pkgIdent, ok := selector.X.(*ast.Ident)
	if !ok || pkgIdent.Name != "fmt" || selector.Sel == nil || selector.Sel.Name != "Sprintf" {
		return "", fmt.Errorf("only fmt.Sprintf is supported")
	}
	if len(call.Args) == 0 {
		return "", fmt.Errorf("fmt.Sprintf requires a format argument")
	}
	format, err := evaluateStringExpr(pkg, file, call.Args[0])
	if err != nil {
		return "", err
	}
	args := make([]any, 0, len(call.Args)-1)
	for _, argExpr := range call.Args[1:] {
		arg, err := evaluateStringExpr(pkg, file, argExpr)
		if err != nil {
			return "", err
		}
		args = append(args, arg)
	}
	return fmt.Sprintf(format, args...), nil
}

func evaluateIdentString(pkg *packages.Package, file *ast.File, ident *ast.Ident) (string, error) {
	if ident == nil {
		return "", fmt.Errorf("nil identifier")
	}
	if ident.Obj != nil {
		switch ident.Obj.Kind {
		case ast.Con:
			return evaluateValueSpecString(pkg, file, ident.Obj.Decl)
		case ast.Var:
			return evaluateValueSpecString(pkg, file, ident.Obj.Decl)
		}
	}
	obj := pkg.TypesInfo.Uses[ident]
	if obj == nil {
		obj = pkg.TypesInfo.Defs[ident]
	}
	if obj == nil {
		return "", fmt.Errorf("unresolved identifier %q", ident.Name)
	}
	if c, ok := obj.(*types.Const); ok {
		return constantStringValue(c.Val())
	}
	if obj.Pkg() == nil || obj.Pkg().Path() != pkg.PkgPath {
		return "", fmt.Errorf("identifier %q must be declared in the same package", ident.Name)
	}
	decl := findObjectDecl(pkg, ident.Name)
	if decl == nil {
		return "", fmt.Errorf("could not resolve identifier %q", ident.Name)
	}
	return evaluateValueSpecString(pkg, file, decl)
}

func evaluateSelectorString(pkg *packages.Package, file *ast.File, selector *ast.SelectorExpr) (string, error) {
	obj := pkg.TypesInfo.Uses[selector.Sel]
	if obj == nil {
		return "", fmt.Errorf("unresolved selector %q", selector.Sel.Name)
	}
	if c, ok := obj.(*types.Const); ok {
		return constantStringValue(c.Val())
	}
	if obj.Pkg() == nil || obj.Pkg().Path() != pkg.PkgPath {
		return "", fmt.Errorf("selector %q is not a same-package constant/var", selector.Sel.Name)
	}
	decl := findObjectDecl(pkg, selector.Sel.Name)
	if decl == nil {
		return "", fmt.Errorf("could not resolve selector %q", selector.Sel.Name)
	}
	return evaluateValueSpecString(pkg, file, decl)
}

func evaluateValueSpecString(pkg *packages.Package, file *ast.File, decl any) (string, error) {
	spec, ok := decl.(*ast.ValueSpec)
	if !ok {
		return "", fmt.Errorf("unsupported declaration type %T", decl)
	}
	if len(spec.Values) == 0 {
		return "", fmt.Errorf("declaration has no value")
	}
	if len(spec.Values) == 1 {
		return evaluateStringExpr(pkg, file, spec.Values[0])
	}
	return "", fmt.Errorf("multi-value declarations are not supported")
}

func findObjectDecl(pkg *packages.Package, name string) any {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, ident := range valueSpec.Names {
					if ident.Name == name {
						return valueSpec
					}
				}
			}
		}
	}
	return nil
}

func constantStringValue(value constant.Value) (string, error) {
	if value.Kind() != constant.String {
		return "", fmt.Errorf("constant is not a string")
	}
	return constant.StringVal(value), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func packageQualifier(pkg *packages.Package) types.Qualifier {
	return func(other *types.Package) string {
		if other == nil || pkg == nil || other.Path() == pkg.PkgPath {
			return ""
		}
		return other.Name()
	}
}

func buildInputPlan(inputType types.Type) (ir.InputPlan, error) {
	st, err := structTypeOf(inputType)
	if err != nil {
		return ir.InputPlan{}, err
	}

	var plan ir.InputPlan
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		if !field.Exported() {
			continue
		}
		tag := reflect.StructTag(st.Tag(i)).Get("gen-router")
		parsed, err := parseTag(tag)
		if err != nil {
			return ir.InputPlan{}, fmt.Errorf("field %s: %w", field.Name(), err)
		}
		source := parsed["in"]
		if source == "" {
			continue
		}
		fieldPlan := ir.InputFieldPlan{
			GoName: field.Name(),
			Source: source,
			Name:   firstNonEmpty(parsed["name"], field.Name()),
			Type:   types.TypeString(field.Type(), types.RelativeTo(field.Pkg())),
		}
		if source == "body" {
			if plan.Body != nil {
				return ir.InputPlan{}, fmt.Errorf("multiple body fields are not supported")
			}
			bodyField := fieldPlan
			plan.Body = &bodyField
			continue
		}
		plan.Fields = append(plan.Fields, fieldPlan)
	}
	return plan, nil
}

func buildOutputPlan(outputType types.Type) (ir.OutputPlan, error) {
	st, err := structTypeOf(outputType)
	if err != nil {
		return ir.OutputPlan{}, err
	}

	plan := ir.OutputPlan{}
	variants := map[int]*ir.OutputVariantPlan{}
	order := []int{}
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		if !field.Exported() {
			continue
		}
		tag := reflect.StructTag(st.Tag(i)).Get("gen-router")
		parsed, err := parseTag(tag)
		if err != nil {
			return ir.OutputPlan{}, fmt.Errorf("field %s: %w", field.Name(), err)
		}
		source := parsed["in"]
		responseCodeRaw := parsed["response"]
		if source == "" && responseCodeRaw == "" {
			continue
		}
		if source == "" {
			source = "body"
		}
		fieldPlan := ir.OutputFieldPlan{
			GoName: field.Name(),
			Source: source,
			Name:   firstNonEmpty(parsed["name"], field.Name()),
			Type:   types.TypeString(field.Type(), types.RelativeTo(field.Pkg())),
		}
		if responseCodeRaw == "" {
			plan.SharedFields = append(plan.SharedFields, fieldPlan)
			continue
		}
		statusCode, err := strconv.Atoi(responseCodeRaw)
		if err != nil {
			return ir.OutputPlan{}, fmt.Errorf("field %s: invalid response code %q", field.Name(), responseCodeRaw)
		}
		variant := variants[statusCode]
		if variant == nil {
			variant = &ir.OutputVariantPlan{StatusCode: statusCode}
			variants[statusCode] = variant
			order = append(order, statusCode)
		}
		variant.Fields = append(variant.Fields, fieldPlan)
	}

	for _, code := range order {
		plan.Variants = append(plan.Variants, *variants[code])
	}
	return plan, nil
}

func structTypeOf(t types.Type) (*types.Struct, error) {
	for {
		switch current := t.(type) {
		case *types.Named:
			t = current.Underlying()
		case *types.Pointer:
			t = current.Elem()
		case *types.Struct:
			return current, nil
		default:
			return nil, fmt.Errorf("type %s is not a struct", types.TypeString(t, nil))
		}
	}
}

func namedType(t types.Type) *types.Named {
	for {
		switch current := t.(type) {
		case *types.Named:
			return current
		case *types.Pointer:
			t = current.Elem()
		default:
			return nil
		}
	}
}

func receiverTypeName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return receiverTypeName(value.X)
	default:
		return ""
	}
}

func splitEndpoint(endpoint string) (string, string, error) {
	method, path, found := strings.Cut(strings.TrimSpace(endpoint), " ")
	if !found || method == "" || strings.TrimSpace(path) == "" {
		return "", "", fmt.Errorf("EndpointPath must look like 'METHOD /path', got %q", endpoint)
	}
	return method, strings.TrimSpace(path), nil
}

func parseTag(raw string) (map[string]string, error) {
	result := map[string]string{}
	if raw == "" {
		return result, nil
	}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, found := strings.Cut(part, ":")
		if !found {
			return nil, fmt.Errorf("invalid gen-router tag part %q", part)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			return nil, fmt.Errorf("invalid gen-router tag part %q", part)
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate gen-router tag key %q", key)
		}
		result[key] = value
	}
	return result, nil
}
