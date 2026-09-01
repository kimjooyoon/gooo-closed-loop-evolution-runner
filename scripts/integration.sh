#!/usr/bin/env bash
set -euo pipefail

evidence_dir=${1:?evidence directory is required}
evidence="$evidence_dir/evidence.json"
generated="$evidence_dir/cases/case-01-one-bug-repair/generated/next-generation/generated_normalization_fix.go"
test -f "$evidence"
test -f "$generated"

jq -e '
  .cases[] | select(.id == "case-01-one-bug-repair") |
  .baseline.status == "FAIL" and .evolved.status == "PASS" and
  .bootstrap.status == "PASS" and .stages[4].terminal_reason == "NEXT_GENERATION_BUILT"
' "$evidence" >/dev/null
grep -q 'normalizationMode = "upper"' "$generated"
printf 'integration witness: generated compiler behavior and terminal records verified\n'
