#!/bin/bash
export KUBEBUILDER_ASSETS="$(pwd)/bin/k8s/1.25.0-darwin-amd64"

RUNS=10
FAILURES=0

for i in $( seq 1 $RUNS )
do
  echo "Run $i/$RUNS ($FAILURES failures so far)"
  go clean -testcache
  go test ./controllers/control/... -count=1
  if [ $? -ne 0 ]; then
    FAILURES=$((FAILURES+1))
  fi
done

echo "Done ($FAILURES failures)"

