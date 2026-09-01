#!/usr/bin/env bash
set -euo pipefail

mode=${1:?usage: verify-release-boundary.sh setting|tag TAG COMMIT|release TAG COMMIT}
repo=${GITHUB_REPOSITORY:-kimjooyoon/gooo-closed-loop-evolution-runner}
api_version='2026-03-10'

case "$mode" in
	setting)
		response=$(gh api -H 'Accept: application/vnd.github+json' -H "X-GitHub-Api-Version: $api_version" "repos/$repo/immutable-releases")
		test "$(jq -er '.enabled' <<<"$response")" = true
		;;
	tag)
		tag=${2:?tag mode requires TAG}
		expected_commit=${3:?tag mode requires COMMIT}
		ref=$(gh api -H 'Accept: application/vnd.github+json' -H "X-GitHub-Api-Version: $api_version" "repos/$repo/git/ref/tags/$tag")
		test "$(jq -er '.object.type' <<<"$ref")" = tag
		tag_object=$(jq -er '.object.sha' <<<"$ref")
		tag_data=$(gh api -H 'Accept: application/vnd.github+json' -H "X-GitHub-Api-Version: $api_version" "repos/$repo/git/tags/$tag_object")
		test "$(jq -er '.object.type' <<<"$tag_data")" = commit
		test "$(jq -er '.object.sha' <<<"$tag_data")" = "$expected_commit"
		;;
	release)
		tag=${2:?release mode requires TAG}
		expected_commit=${3:?release mode requires COMMIT}
		response=$(gh api -H 'Accept: application/vnd.github+json' -H "X-GitHub-Api-Version: $api_version" "repos/$repo/releases/tags/$tag")
		test "$(jq -er '.immutable' <<<"$response")" = true
		jq -e '(.assets | length) > 0 and all(.assets[]; (.digest | type == "string" and startswith("sha256:")))' <<<"$response" >/dev/null
		"$0" tag "$tag" "$expected_commit"
		;;
	*)
		echo "unknown mode: $mode" >&2
		exit 2
		;;
esac

printf 'release boundary: %s CLOSED\n' "$mode"

