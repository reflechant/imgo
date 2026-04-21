#!/bin/bash
set -e

# Use awk to calculate total statement coverage for specific files.
# This handles files with multiple functions correctly.
check_coverage() {
  local file=$1
  local expected=$2
  
  if [ ! -f "coverage.out" ]; then
    echo "ERROR: coverage.out not found. Run tests with -coverprofile=coverage.out first."
    exit 1
  fi

  # Use high precision for the comparison
  local actual=$(go tool cover -func=coverage.out | grep "$file" | awk '{sum+=$NF; count++} END {if (count > 0) printf "%.2f", sum/count}')
  local expected_val=$(echo $expected | tr -d '%')
  
  echo "$file coverage: $actual% (expected $expected)"
  if (( $(echo "$actual < $expected_val" | bc -l) )); then
    echo "ERROR: $file must have $expected coverage."
    exit 1
  fi
}

check_coverage "rewrite.go" "100.0%"
check_coverage "validator.go" "100.0%"
