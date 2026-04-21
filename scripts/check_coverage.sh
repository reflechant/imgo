#!/bin/bash
set -e

# Use awk to calculate total statement coverage for specific files.
# This handles files with multiple functions correctly.
check_coverage() {
  local path=$1
  local expected=$2
  
  if [ ! -f "coverage.out" ]; then
    echo "ERROR: coverage.out not found. Run tests with -coverprofile=coverage.out first."
    exit 1
  fi

  # Grep for path prefix to be specific
  local actual=$(go tool cover -func=coverage.out | grep "$path:" | awk '{sum+=$NF; count++} END {if (count > 0) printf "%.2f", sum/count}')
  
  if [ -z "$actual" ]; then
    echo "ERROR: Could not find coverage for $path"
    exit 1
  fi

  local expected_val=$(echo $expected | tr -d '%')
  
  echo "$path coverage: $actual% (expected $expected)"
  if (( $(echo "$actual < $expected_val" | bc -l) )); then
    echo "ERROR: $path must have $expected coverage."
    exit 1
  fi
}

# Demand 100% coverage for all source files in key packages
PACKAGES=("pkg/persistent" "pkg/transpiler")

for pkg in "${PACKAGES[@]}"; do
  echo "Checking coverage for $pkg..."
  # Find all .go files that are not tests
  for f in $(ls $pkg/*.go | grep -v "_test.go"); do
    check_coverage "$f" "100.0%"
  done
done
