.PHONY: bench bench-save

BENCH_PKGS := ./tests/gen_bench ./tests/std_http_bench
BENCH_CMD := go test $(BENCH_PKGS) -run '^$$' -bench . -benchmem -count=1

bench:
	@echo "Running benchmarks..."
	@$(BENCH_CMD)

bench-save:
	@echo "Saving benchmark results..."
	@mkdir -p bench-results
	@$(BENCH_CMD) | tee "bench-results/bench-$$(date +%Y%m%d-%H%M%S).txt"
