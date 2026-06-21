package genbench

import (
	"net/http/httptest"
	"testing"

	"github.com/P4elme6ka/gen-router/internal/render"
)

type benchmarkRenderOutput struct {
	RequestID string                  `gen-router:"in:header;name:X-Request-Id"`
	Success   *benchmarkRenderSuccess `gen-router:"response:200;in:body"`
	Failure   *benchmarkRenderFailure `gen-router:"response:500;in:body"`
}

type benchmarkRenderSuccess struct {
	Message string `json:"message"`
}

type benchmarkRenderFailure struct {
	Error string `json:"error"`
}

func benchmarkOutputFixture() benchmarkRenderOutput {
	return benchmarkRenderOutput{
		RequestID: "req-bench",
		Success:   &benchmarkRenderSuccess{Message: "benchmark"},
	}
}

func BenchmarkWriteOutput(b *testing.B) {
	output := benchmarkOutputFixture()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		if err := render.WriteOutput(recorder, output); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompilePlanWriteOutput(b *testing.B) {
	output := benchmarkOutputFixture()
	plan, err := render.Compile(benchmarkRenderOutput{})
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
