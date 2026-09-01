#!/usr/bin/env bash
set -euo pipefail

evidence_dir=${1:?evidence directory is required}
repo_root=$(cd "$(dirname "$0")/.." && pwd)
meta="$repo_root/.gooo/closed-loop-evolution-runner.gooo"
evidence="$evidence_dir/evidence.json"
test -f "$meta"
test -f "$evidence"
bash "$repo_root/scripts/validate-evidence.sh" "$evidence"

grep -q '^precedence REFUTED>UNKNOWN>CLOSED$' "$meta"
grep -q '^unknown_fields stage,step,reason,unknown_class,next_operation,blocked_by$' "$meta"
grep -q '^atomic_abort states=UNKNOWN,REFUTED promote_artifact=false partial_promotion=false$' "$meta"
test "$(grep -c '^stage ordinal=' "$meta")" = 8

jq -e '
	.schema == "gooo/closed-loop-evolution-runner/evidence/v2" and
	.fixed_case_count == 5 and .fixed_stage_count == 8 and
	.summary == {generated:5,closed:2,unknown:2,refuted:1} and
	([.cases[] | select(.state == "UNKNOWN") | .unknown | keys | sort] | all(. == ["blocked_by","next_operation","reason","stage","step","unknown_class"])) and
	([.cases[] | select(.id == "case-01-one-bug-repair") | select(.state == "CLOSED" and .baseline.status == "FAIL" and .evolved.status == "PASS" and .bootstrap.status == "PASS" and .promoted_artifact == true)] | length) == 1 and
	([.cases[] | select(.id == "case-03-semantic-drift") | select(.state == "REFUTED" and .promoted_artifact == false)] | length) == 1 and
	([.cases[] | select(.id == "case-05-byte-identical-replay") | select(.state == "CLOSED" and .replay_equal == true)] | length) == 1 and
	([.cases[].stages[] | select(((.input_digest | startswith("sha256:")) | not) or ((.output_digest | startswith("sha256:")) | not) or .capability == "" or .terminal_reason == "" or .next_operation == "")] | length) == 0 and
	.authority.repository_writes == 0 and .authority.commit_authority == 0 and .authority.merge_authority == 0 and .authority.release_authority == 0
' "$evidence"

if rg -n 'git (commit|merge|push|reset|checkout)|gh (pr merge|release delete)|rm -rf /' "$repo_root/cmd" "$repo_root/internal"; then
	echo 'semantic audit: repository mutation authority found' >&2
	exit 1
fi
printf 'semantic audit: CLOSED vector, UNKNOWN frontier, REFUTED guard, and zero mutation authority verified\n'
