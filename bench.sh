#!/bin/bash
set -euo pipefail

# We run each benchmark 20 times total (10x2) for 0.5 sec each, with some interleaving of the benchmarks.
benchloops=10
benchcount=2
benchtime=0.5s

# Ask to use more modern CPU instructions on amd64.
goarch=$(go env GOARCH)
if [[ "$goarch" == "amd64" ]]; then
  export GOAMD64=v3
  echo GOAMD64="$GOAMD64"
fi

# Execute the interleaved benchmarks, with swiss.Map first.
go test -c
echo "Available benchmarks:"
go test -list=^Bench
rm -f out
for i in $(seq $benchloops); do
  for impl in swissMap runtimeMap; do
    # Manually get the list of benchmarks.
    # The next line can be edited to control the order of results, such as 'for benchmark in MapPutGrow MapGetHit; do'
    for benchmark in $(go test -list=^Bench | grep '^Bench'); do
      ./swiss.test -test.v -test.run=NONE -test.bench="$benchmark"/impl="$impl" -test.count="$benchcount" \
        -test.benchtime="$benchtime" -test.benchmem -test.timeout=10h 2>&1 | tee -a out
    done
  done
done

# Compare the results.
grep -v swissMap out | egrep -v '^Benchmark[^ ]+$' | sed 's,/impl=runtimeMap,,g' > out.runtime
grep -v runtimeMap out | egrep -v '^Benchmark[^ ]+$' | sed 's,/impl=swissMap,,g' > out.swiss
benchstat out.runtime out.swiss 2>&1 | tee out.benchstat
