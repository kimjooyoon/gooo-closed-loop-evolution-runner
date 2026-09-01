#!/usr/bin/env bash
set -euo pipefail

current_dir=${1:?current evidence directory is required}
repo=${GITHUB_REPOSITORY:-kimjooyoon/gooo-closed-loop-evolution-runner}
baseline_tmp=$(mktemp -d "${RUNNER_TEMP:-/tmp}/gooo-v010-baseline.XXXXXX")
cleanup() {
	rm -rf "$baseline_tmp"
}
trap cleanup EXIT

release=$(gh api "repos/$repo/releases/tags/v0.1.0")
test "$(jq -er '.immutable' <<<"$release")" = true
gh release download v0.1.0 --repo "$repo" --pattern 'gooo-closed-loop-evolution-runner-evidence-v0.1.0.tar.gz' --dir "$baseline_tmp" >/dev/null
mkdir -p "$baseline_tmp/extracted"
tar -xzf "$baseline_tmp/gooo-closed-loop-evolution-runner-evidence-v0.1.0.tar.gz" -C "$baseline_tmp/extracted"
baseline="$baseline_tmp/extracted/evidence.json"
current="$current_dir/evidence.json"
test -f "$baseline"
test -f "$current"

projection() {
	jq -S '
		def terminal: {command,status,exit_code,terminal_reason};
		{
			contract_digest,
			toolchain_digest,
			fixed_case_count,
			fixed_stage_count,
			precedence,
			unknown_fields,
			summary,
			tools: [.tools[] | {id,repository,release,release_id,immutable,asset,expected_digest,observed_digest,available,reason}],
			cases: [.cases[] | {
				id,state,decision,candidate_ids,candidate_digests,promoted_artifact,replay_equal,atomic_abort,
				baseline:(.baseline|terminal),
				build:(.build|terminal),
				evolved:(.evolved|terminal),
				bootstrap:(.bootstrap|terminal),
				generated,
				replay_generated,
				stages:[.stages[] | {ordinal,id,capability,terminal_reason,next_operation}]
			}]
		}
	' "$1"
}

projection "$baseline" > "$baseline_tmp/baseline-projection.json"
projection "$current" > "$baseline_tmp/current-projection.json"
cmp -s "$baseline_tmp/baseline-projection.json" "$baseline_tmp/current-projection.json"

for artifact in \
	"case-01-one-bug-repair/generated/next-generation/generated_normalization_fix.go" \
	"case-05-byte-identical-replay/generated/next-generation/generated_normalization_fix.go" \
	"case-05-byte-identical-replay/generated/replay/generated_normalization_fix.go"; do
	test -f "$current_dir/cases/$artifact"
	test -f "$baseline_tmp/extracted/cases/$artifact"
	cmp -s "$current_dir/cases/$artifact" "$baseline_tmp/extracted/cases/$artifact"
done

printf 'v0.1.0 preservation: case terminals, immutable tool locks, generated artifacts, and replay bytes identical\n'
