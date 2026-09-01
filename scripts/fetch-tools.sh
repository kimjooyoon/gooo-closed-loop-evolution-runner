#!/usr/bin/env bash
set -euo pipefail

lock_path=${1:?immutable tool lock path is required}
output_dir=${2:?caller-owned tool directory is required}
mkdir -p "$output_dir"

rm -f "$output_dir"/.tool-status-*.json "$output_dir/tool-fetch-manifest.json"

while IFS= read -r tool; do
	id=$(jq -er '.id' <<<"$tool")
	repository=$(jq -er '.repository' <<<"$tool")
	tag=$(jq -er '.tag' <<<"$tool")
	tag_object_expected=$(jq -er '.tag_object' <<<"$tool")
	target_expected=$(jq -er '.target_commit' <<<"$tool")
	asset=$(jq -er '.asset.name' <<<"$tool")
	digest_expected=$(jq -er '.asset.digest' <<<"$tool")
	status_file="$output_dir/.tool-status-${id//@/_}.json"
	reason=IMMUTABLE_TOOL_EVIDENCE_UNAVAILABLE
	available=false
	observed_digest=""
	api_response=""

	if api_response=$(gh api -H 'Accept: application/vnd.github+json' -H 'X-GitHub-Api-Version: 2026-03-10' "repos/$repository/releases/tags/$tag" 2>/dev/null); then
		immutable=$(jq -r 'if has("immutable") then (.immutable|tostring) else "missing" end' <<<"$api_response")
		release_id=$(jq -r '.id // 0' <<<"$api_response")
		api_digest=$(jq -r --arg asset "$asset" '.assets[] | select(.name == $asset) | (.digest // "missing")' <<<"$api_response" | head -n 1)
		ref_response=$(gh api -H 'Accept: application/vnd.github+json' -H 'X-GitHub-Api-Version: 2026-03-10' "repos/$repository/git/ref/tags/$tag" 2>/dev/null || true)
		tag_object=$(jq -r '.object.sha // "missing"' <<<"$ref_response")
		tag_type=$(jq -r '.object.type // "missing"' <<<"$ref_response")
		tag_data=""
		if [ "$tag_type" = tag ]; then
			tag_data=$(gh api -H 'Accept: application/vnd.github+json' -H 'X-GitHub-Api-Version: 2026-03-10' "repos/$repository/git/tags/$tag_object" 2>/dev/null || true)
		fi
		target=$(jq -r '.object.sha // "missing"' <<<"$tag_data")
		if [ "$immutable" = true ] && [ "$release_id" = "$(jq -er '.release_id' <<<"$tool")" ] && [ "$tag_object" = "$tag_object_expected" ] && [ "$target" = "$target_expected" ] && [ "$api_digest" = "$digest_expected" ]; then
			url="https://github.com/$repository/releases/download/$tag/$asset"
			if curl --fail --location --retry 3 --silent --show-error "$url" -o "$output_dir/$asset"; then
				observed_digest="sha256:$(sha256sum "$output_dir/$asset" | awk '{print $1}')"
				if [ "$observed_digest" = "$digest_expected" ]; then
					available=true
					reason=OK
				else
					reason=IMMUTABLE_TOOL_DIGEST_MISMATCH
				fi
			else
				reason=IMMUTABLE_TOOL_EVIDENCE_MISSING
			fi
		else
			reason=IMMUTABLE_TOOL_CONTRACT_MISMATCH
		fi
	else
		reason=IMMUTABLE_TOOL_RELEASE_API_UNAVAILABLE
	fi

	jq -n -c --arg id "$id" --arg reason "$reason" --arg observed "$observed_digest" --arg expected "$digest_expected" --argjson available "$available" '{id:$id,available:$available,reason:$reason,expected_digest:$expected,observed_digest:$observed}' > "$status_file"
done < <(jq -c '.tools[]' "$lock_path")

jq -s 'sort_by(.id)' "$output_dir"/.tool-status-*.json > "$output_dir/tool-fetch-manifest.json"
rm -f "$output_dir"/.tool-status-*.json
printf 'immutable tool fetch complete: %s\n' "$(jq 'map(select(.available == true)) | length' "$output_dir/tool-fetch-manifest.json")/$(jq 'length' "$output_dir/tool-fetch-manifest.json") available"

