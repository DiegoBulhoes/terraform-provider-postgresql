#!/usr/bin/env bash
# Usage: coverage.sh FLOOR PROFILE...
# Merges coverage profiles of the same mode and fails when the total is below FLOOR percent.
set -euo pipefail

floor=$1
shift
merged=$(mktemp)
trap 'rm -f "$merged"' EXIT

# A manual CI run without PostgreSQL 17 has no acceptance profile; the others still count.
profiles=()
for profile in "$@"; do
  if [ -f "$profile" ]; then
    profiles+=("$profile")
  else
    echo "::warning::coverage profile $profile not found, skipped"
  fi
done
if [ ${#profiles[@]} -eq 0 ]; then
  echo "::error::no coverage profile found"
  exit 1
fi

head -1 "${profiles[0]}" >"$merged"
for profile in "${profiles[@]}"; do
  tail -n +2 "$profile" >>"$merged"
done

# cover sums the counts of a block that appears in several profiles.
total=$(go tool cover -func="$merged" | awk '/^total:/ { sub("%", "", $NF); print $NF }')
echo "Coverage of ./internal/...: ${total}% (floor ${floor}%)"
awk -v t="$total" -v f="$floor" 'BEGIN { exit !(t >= f) }' || {
  echo "::error::coverage ${total}% is below the ${floor}% floor"
  exit 1
}
