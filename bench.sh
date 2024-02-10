#!/bin/bash

go test -v -run - -bench . -count 10 -timeout 1h | tee out
grep -v swissMap out | sed 's,runtimeMap,Map,g' > out.runtime
grep -v runtimeMap out | sed 's,swissMap,Map,g' > out.swiss
benchstat out.runtime out.swiss
