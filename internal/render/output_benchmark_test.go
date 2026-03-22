package render

import (
	"net/http/httptest"
	"testing"
)

func benchmarkRenderOutput() renderOutput {
	return renderOutput{
		RequestID: "req-bench",
		Success:   &renderSuccess{Message: "benchmark"},
	}
}

func BenchmarkWriteOutput(b *testing.B) {
	output := benchmarkRenderOutput()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		if err := WriteOutput(recorder, output); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompilePlanWriteOutput(b *testing.B) {
	output := benchmarkRenderOutput()
	plan, err := Compile(renderOutput{})
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		if err := plan.Write(recorder, output); err != nil {
			b.Fatal(err)
		}
	}
}
