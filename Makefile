.PHONY: bench bench-save

BENCH_PKGS := ./internal/bind ./internal/render ./
BENCH_CMD := go test $(BENCH_PKGS) -run '^$$' -bench . -benchmem -count=1

bench:
	@echo "Running benchmarks..."
	@$(BENCH_CMD)

bench-save:
	@echo "Saving benchmark results..."
	@mkdir -p bench-results
	@$(BENCH_CMD) | tee "bench-results/bench-$$(date +%Y%m%d-%H%M%S).txt"
