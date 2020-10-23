#!/bin/bash

function benchfuncs {
	for t in $(go test -test.list 'Benchmark.*'); do
		if [[ "${t}" == "ok" ]]; then return; fi
		printf "%s\n" "$t"
	done
}

for t in $(benchfuncs); do
	#printf ":%q:\n" "$t"
	args=()
	args+=("-run=XXX")                             # don't run other tests
	args+=("-bench=^${t}\$")                       # run precisely this one bench func
	args+=("-o" ".tmp-prof/${t}.bench.bin")        # save the binary (needed for pprof later)
	args+=("-cpuprofile=.tmp-prof/${t}.cpu.pprof") # save the cpu profile
	#args+=("-gcflags" "-S")                       # ask compiler to emit assembly
	go test "${args[@]}" | grep "^Benchmark_"
done

for t in $(benchfuncs); do
	go tool pprof \
		--pdf \
		--output ".tmp-prof/${t}.cpu.pdf" \
		".tmp-prof/${t}.bench.bin" ".tmp-prof/${t}.cpu.pprof"
done

#
# go build -gcflags "-m"
#      this is supposed to tell you about escape analysis and whether each var gets heap or stack alloc.
#
