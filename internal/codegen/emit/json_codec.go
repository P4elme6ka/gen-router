package emit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/P4elme6ka/gen-json/pkg/genjson"
	"github.com/P4elme6ka/gen-router/internal/codegen/ir"
)

// GenerateJSONCodec generates fast JSON encoder/decoder code for input body types.
func GenerateJSONCodec(pkg ir.PackagePlan) error {
	bodyTypes := make(map[string]bool)
	for _, handler := range pkg.Handlers {
		if handler.Input.Body != nil {
			// Extract simple type name (e.g., "CustomerCreateBody" from "main.CustomerCreateBody")
			typeName := handler.Input.Body.Type
			if strings.Contains(typeName, ".") {
				parts := strings.Split(typeName, ".")
				typeName = parts[len(parts)-1]
			}
			bodyTypes[typeName] = true
		}
	}
	if len(bodyTypes) == 0 {
		return nil
	}
	types := make([]string, 0, len(bodyTypes))
	for t := range bodyTypes {
		types = append(types, t)
	}
	cfg := genjson.Config{
		PackageDir:    pkg.Dir,
		Output:        filepath.Join(pkg.Dir, JSONCodecFileName()),
		Types:         types,
		Features:      []string{genjson.FeatureUnknownFields},
		EmitMarshaler: false,
	}
	return writeJSONCodec(cfg)
}

// writeJSONCodec generates and writes JSON codec file using gen-json.
func writeJSONCodec(cfg genjson.Config) error {
	code, err := genjson.Generate(cfg)
	if err != nil {
		return fmt.Errorf("generate JSON codec: %w", err)
	}
	if err := os.WriteFile(cfg.Output, code, 0o644); err != nil {
		return fmt.Errorf("write JSON codec file: %w", err)
	}
	return nil
}

// JSONCodecFileName returns the standard output filename for JSON codecs.
func JSONCodecFileName() string {
	return "zz_gen_router_json.go"
}

// JSONCodecOutputPath returns the full output path for JSON codec file.
func JSONCodecOutputPath(pkg ir.PackagePlan) string {
	return filepath.Join(pkg.Dir, JSONCodecFileName())
}
