#!/usr/bin/env bash
set -euo pipefail

binary=${1:?path to the built runner binary is required}
metrics_file=${2:?CI metrics file is required}
root=$(pwd)
work=$(mktemp -d "${RUNNER_TEMP:-/tmp}/gooo-closed-loop-evolution-runner.XXXXXX")
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/fixture" "$work/output" "$work/tools" "$work/second-fixture" "$work/second-output" "$work/second-tools"

bash scripts/fetch-tools.sh contracts/immutable-tool-lock-v1.json "$work/tools"
bash scripts/fetch-tools.sh contracts/immutable-tool-lock-v1.json "$work/second-tools"

before=$(git status --porcelain=v1 -z --untracked-files=all | sha256sum | awk '{print $1}')
"$binary" run \
	--source .gooo/closed-loop-evolution-runner.gooo \
	--contract contracts/denominator-v1.json \
	--tool-lock contracts/immutable-tool-lock-v1.json \
	--candidates-root examples/candidates \
	--fixture-root "$work/fixture" \
	--tools-dir "$work/tools" \
	--out "$work/output" \
	--source-root "$root"
after=$(git status --porcelain=v1 -z --untracked-files=all | sha256sum | awk '{print $1}')
test "$before" = "$after"

"$binary" run \
	--source .gooo/closed-loop-evolution-runner.gooo \
	--contract contracts/denominator-v1.json \
	--tool-lock contracts/immutable-tool-lock-v1.json \
	--candidates-root examples/candidates \
	--fixture-root "$work/second-fixture" \
	--tools-dir "$work/second-tools" \
	--out "$work/second-output" \
	--source-root "$root" >/dev/null

jq -e '
	.schema == "gooo/closed-loop-evolution-runner/evidence/v2" and
	.fixed_case_count == 5 and .fixed_stage_count == 8 and
	.precedence == ["REFUTED", "UNKNOWN", "CLOSED"] and
	.summary == {generated:5,closed:2,unknown:2,refuted:1} and
	.authority == {repository_writes:0,commit_authority:0,merge_authority:0,release_authority:0} and
	.metrics.inventory.root_readme_excluded == true and
	.metrics.inventory.go_files > 0 and .metrics.inventory.gooo_files > 0 and
	.metrics.inventory.go_physical_lines > 0 and .metrics.inventory.gooo_physical_lines > 0 and
	.metrics.inventory.physical_lines > 0 and .metrics.inventory.descendant_dirs > 0 and
	.metrics.inventory.regular_files > 0 and .metrics.wall_ms > 0 and .metrics.peak_rss_kib > 0 and
	.metrics.tests == {total:5,selected:3,executed:6,reused:1,failed:1,unknown:2} and
	([.tools[] | select(.available == true)] | length) == 5 and
	([.cases[] | select(.stages | length != 8)] | length) == 0 and
	([.cases[] | select(.state == "CLOSED" and .promoted_artifact == true and (.generated.files | length) > 0)] | length) == 2 and
	([.cases[] | select(.id == "case-01-one-bug-repair") | (.baseline.status == "FAIL" and .evolved.status == "PASS" and .bootstrap.status == "PASS" and .improvement.state == "CLOSED" and (.generated.files | length) == 1)] | all) and
	([.cases[] | select(.id == "case-02-ambiguous-candidates") | (.state == "UNKNOWN" and (.unknown | keys | sort) == ["blocked_by","next_operation","reason","stage","step","unknown_class"])] | all) and
	([.cases[] | select(.id == "case-03-semantic-drift") | (.state == "REFUTED" and .promoted_artifact == false and .evolved.status == "FAIL" and .decision == "SEMANTIC_DRIFT")] | all) and
	([.cases[] | select(.id == "case-04-missing-immutable-tool") | (.state == "UNKNOWN" and .unknown.unknown_class == "TOOL_CONTRACT_MISMATCH")] | all) and
	([.cases[] | select(.id == "case-05-byte-identical-replay") | (.state == "CLOSED" and .replay_equal == true and .improvement.state == "CLOSED")] | all) and
	([.cases[] | select(.id == "case-01-one-bug-repair") | {tests_total,tests_selected,tests_executed,tests_reused,tests_failed,tests_unknown} == {tests_total:1,tests_selected:1,tests_executed:2,tests_reused:0,tests_failed:0,tests_unknown:0}] | length) == 1 and
	([.cases[] | select(.id == "case-02-ambiguous-candidates") | {tests_total,tests_selected,tests_executed,tests_reused,tests_failed,tests_unknown} == {tests_total:1,tests_selected:0,tests_executed:0,tests_reused:0,tests_failed:0,tests_unknown:1}] | length) == 1 and
	([.cases[] | select(.id == "case-03-semantic-drift") | {tests_total,tests_selected,tests_executed,tests_reused,tests_failed,tests_unknown} == {tests_total:1,tests_selected:1,tests_executed:1,tests_reused:0,tests_failed:1,tests_unknown:0}] | length) == 1 and
	([.cases[] | select(.id == "case-04-missing-immutable-tool") | {tests_total,tests_selected,tests_executed,tests_reused,tests_failed,tests_unknown} == {tests_total:1,tests_selected:0,tests_executed:0,tests_reused:0,tests_failed:0,tests_unknown:1}] | length) == 1 and
	([.cases[] | select(.id == "case-05-byte-identical-replay") | {tests_total,tests_selected,tests_executed,tests_reused,tests_failed,tests_unknown} == {tests_total:1,tests_selected:1,tests_executed:3,tests_reused:1,tests_failed:0,tests_unknown:0}] | length) == 1 and
	([.cases[].stages[] | select(.input_digest == "" or .output_digest == "" or .capability == "" or .terminal_reason == "" or .next_operation == "")] | length) == 0
' "$work/output/evidence.json"

jq -S 'del(.metrics, .artifact_count, .cases[].stages[].wall_ms, .cases[].stages[].peak_rss_kib, .cases[].baseline.stdout, .cases[].baseline.stderr, .cases[].baseline.output_digest, .cases[].evolved.stdout, .cases[].evolved.stderr, .cases[].evolved.output_digest, .cases[].bootstrap.stdout, .cases[].bootstrap.stderr, .cases[].bootstrap.output_digest, .cases[].build.stdout, .cases[].build.stderr, .cases[].build.output_digest)' "$work/output/evidence.json" > "$work/first-semantic.json"
jq -S 'del(.metrics, .artifact_count, .cases[].stages[].wall_ms, .cases[].stages[].peak_rss_kib, .cases[].baseline.stdout, .cases[].baseline.stderr, .cases[].baseline.output_digest, .cases[].evolved.stdout, .cases[].evolved.stderr, .cases[].evolved.output_digest, .cases[].bootstrap.stdout, .cases[].bootstrap.stderr, .cases[].bootstrap.output_digest, .cases[].build.stdout, .cases[].build.stderr, .cases[].build.output_digest)' "$work/second-output/evidence.json" > "$work/second-semantic.json"
cmp -s "$work/first-semantic.json" "$work/second-semantic.json"
cmp -s "$work/output/cases/case-01-one-bug-repair/generated/next-generation/generated_normalization_fix.go" "$work/second-output/cases/case-01-one-bug-repair/generated/next-generation/generated_normalization_fix.go"
cmp -s "$work/output/cases/case-05-byte-identical-replay/generated/next-generation/generated_normalization_fix.go" "$work/second-output/cases/case-05-byte-identical-replay/generated/next-generation/generated_normalization_fix.go"

bash scripts/compare-v010.sh "$work/output"
bash scripts/measure-command.sh "$metrics_file" integration bash scripts/integration.sh "$work/output"

rm -rf "$RUNNER_TEMP/gooo-closed-loop-evolution-runner-evidence"
cp -R "$work/output" "$RUNNER_TEMP/gooo-closed-loop-evolution-runner-evidence"
printf 'closed-loop conformance: CLOSED vector and fail-closed frontier verified\n'
