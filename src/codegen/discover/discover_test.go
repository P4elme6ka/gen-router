package discover

import (
	"go/types"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestDiscoverEndpointPath_SupportsLiteralConcatAndSprintf(t *testing.T) {
	pkg := loadTestPackage(t, "./testdata/endpointcases")
	cases := map[string]string{
		"LiteralHandler": "GET /literal",
		"ConcatHandler":  "POST /customers/{id}",
		"SprintfHandler": "POST /v1/customers/{id}",
		"NestedHandler":  "PATCH /items/{id}",
	}

	for handlerName, expected := range cases {
		handler := lookupNamedType(t, pkg, handlerName)
		got, err := discoverEndpointPath(pkg, handler)
		if err != nil {
			t.Fatalf("discoverEndpointPath(%s) returned error: %v", handlerName, err)
		}
		if got != expected {
			t.Fatalf("discoverEndpointPath(%s) = %q, want %q", handlerName, got, expected)
		}
	}
}

func loadTestPackage(t *testing.T, pattern string) *packages.Package {
	t.Helper()
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
	}, pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		t.Fatalf("package load failed: %v", pkg.Errors[0])
	}
	return pkg
}

func lookupNamedType(t *testing.T, pkg *packages.Package, name string) *types.Named {
	t.Helper()
	obj := pkg.Types.Scope().Lookup(name)
	if obj == nil {
		t.Fatalf("type %s not found", name)
	}
	typeName, ok := obj.(*types.TypeName)
	if !ok {
		t.Fatalf("object %s is not a type", name)
	}
	named, ok := typeName.Type().(*types.Named)
	if !ok {
		t.Fatalf("type %s is not named", name)
	}
	return named
}
