package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/P4elme6ka/gen-router/internal/codegen/discover"
	"github.com/P4elme6ka/gen-router/internal/codegen/emit"
	"github.com/P4elme6ka/gen-router/internal/codegen/ir"
)

func main() {
	var output string
	var write bool
	flag.StringVar(&output, "output", "json", "output format: json")
	flag.BoolVar(&write, "write", false, "write generated metadata files into discovered packages")
	flag.Parse()

	plan, err := discover.LoadModulePlan(discover.Config{Patterns: flag.Args()})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if write {
		if err := writeGeneratedMetadata(plan); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	switch output {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(plan); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unsupported output format %q\n", output)
		os.Exit(1)
	}
}

func writeGeneratedMetadata(plan *ir.ModulePlan) error {
	for _, pkg := range plan.Packages {
		if pkg.Dir == "" || len(pkg.Handlers) == 0 {
			continue
		}
		content, err := emit.RenderMetadataFile(pkg)
		if err != nil {
			return err
		}
		if err := os.WriteFile(emit.OutputPath(pkg), content, 0o644); err != nil {
			return err
		}
	}
	return nil
}
