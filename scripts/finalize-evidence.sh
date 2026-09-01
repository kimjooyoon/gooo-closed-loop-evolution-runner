#!/usr/bin/env bash
set -euo pipefail

evidence_dir=${1:?evidence directory is required}
metrics_file=${2:?CI metrics file is required}
evidence="$evidence_dir/evidence.json"
report="$evidence_dir/runner-report.md"
test -f "$evidence"
test -f "$report"
test -f "$metrics_file"

metric_values() {
	local metric_name=$1
	local matches
	matches=$(awk -F '\t' -v name="$metric_name" '$1 == name { print $0 }' "$metrics_file")
	test "$(printf '%s\n' "$matches" | awk 'NF { count++ } END { print count + 0 }')" = 1
	printf '%s\n' "$matches"
}

read_metric() {
	local metric_name=$1
	local record
	record=$(metric_values "$metric_name")
	IFS=$'\t' read -r metric wall rss <<< "$record"
	test "$metric" = "$metric_name"
	case "$wall" in ''|*[!0-9]*) echo "invalid wall_ms for $metric_name" >&2; exit 1 ;; esac
	case "$rss" in ''|*[!0-9]*) echo "invalid peak_rss_kib for $metric_name" >&2; exit 1 ;; esac
	test "$wall" -gt 0
	test "$rss" -gt 0
	printf '%s\t%s' "$wall" "$rss"
}

IFS=$'\t' read -r compile_wall compile_rss <<< "$(read_metric compile)"
IFS=$'\t' read -r build_wall build_rss <<< "$(read_metric build)"
IFS=$'\t' read -r test_wall test_rss <<< "$(read_metric test)"
IFS=$'\t' read -r conformance_wall conformance_rss <<< "$(read_metric conformance)"
IFS=$'\t' read -r integration_wall integration_rss <<< "$(read_metric integration)"

jq \
	--argjson compile_wall "$compile_wall" --argjson compile_rss "$compile_rss" \
	--argjson build_wall "$build_wall" --argjson build_rss "$build_rss" \
	--argjson test_wall "$test_wall" --argjson test_rss "$test_rss" \
	--argjson conformance_wall "$conformance_wall" --argjson conformance_rss "$conformance_rss" \
	--argjson integration_wall "$integration_wall" --argjson integration_rss "$integration_rss" \
	'.metrics.compile_wall_ms = $compile_wall |
	 .metrics.compile_peak_rss_kib = $compile_rss |
	 .metrics.build_wall_ms = $build_wall |
	 .metrics.build_peak_rss_kib = $build_rss |
	 .metrics.test_wall_ms = $test_wall |
	 .metrics.test_peak_rss_kib = $test_rss |
	 .metrics.conformance_wall_ms = $conformance_wall |
	 .metrics.conformance_peak_rss_kib = $conformance_rss |
	 .metrics.integration_wall_ms = $integration_wall |
	 .metrics.integration_peak_rss_kib = $integration_rss |
	 .metrics.local_test_executions = 0 |
	 .metrics.local_build_executions = 0 |
	 .metrics.local_vet_executions = 0 |
	 .metrics.local_conformance_executions = 0 |
	 .metrics.local_integration_executions = 0' \
	"$evidence" > "$evidence.tmp"
mv "$evidence.tmp" "$evidence"

ci_line="- CI compile/build/test/conformance/integration wall_ms: ${compile_wall}/${build_wall}/${test_wall}/${conformance_wall}/${integration_wall}; peak_rss_kib: ${compile_rss}/${build_rss}/${test_rss}/${conformance_rss}/${integration_rss}"
local_line='- local test/build/vet/conformance/integration executions: `0`/`0`/`0`/`0`/`0`'
awk -v ci="$ci_line" -v local="$local_line" '
	/^- CI compile\/build\/test\/conformance\/integration wall_ms:/ { print ci; next }
	/^- local test\/build\/vet\/conformance\/integration executions:/ { print local; next }
	{ print }
' "$report" > "$report.tmp"
mv "$report.tmp" "$report"

bash "$(dirname "$0")/validate-evidence.sh" "$evidence"

for mutation in missing null string; do
	mutated="$evidence_dir/evidence-invalid-$mutation.json"
	case "$mutation" in
		missing) jq 'del(.metrics.inventory.go_physical_lines)' "$evidence" > "$mutated" ;;
		null) jq '.metrics.compile_wall_ms = null' "$evidence" > "$mutated" ;;
		string) jq '.cases[0].tests_total = "1"' "$evidence" > "$mutated" ;;
	esac
	if bash "$(dirname "$0")/validate-evidence.sh" "$mutated" >/dev/null 2>&1; then
		echo "invalid $mutation evidence was accepted" >&2
		exit 1
	fi
	rm -f "$mutated"
done

printf 'evidence finalized: measured CI fields injected and invalid type/missing-field probes rejected\n'
