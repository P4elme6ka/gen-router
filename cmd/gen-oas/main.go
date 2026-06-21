package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/P4elme6ka/gen-router/src/codegen/discover"
	"github.com/P4elme6ka/gen-router/src/oas"
)

func main() {
	var output string
	flag.StringVar(&output, "output", "openapi.json", "output file path")
	flag.Parse()

	plan, err := discover.LoadModulePlan(discover.Config{Patterns: flag.Args()})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	spec := oas.GenerateSpec(plan)

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(spec); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if output != "" {
		data, _ := json.MarshalIndent(spec, "", "  ")
		if err := os.WriteFile(output, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write %s: %v\n", output, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "wrote OpenAPI spec to %s\n", output)
	}
}
