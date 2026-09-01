#!/usr/bin/env bash
set -euo pipefail

metrics_file=${1:?metrics output file is required}
metric_name=${2:?metric name is required}
shift 2
test "$#" -gt 0

mkdir -p "$(dirname "$metrics_file")"
stats_file=$(mktemp "${RUNNER_TEMP:-/tmp}/gooo-command-stats.XXXXXX")
cleanup() {
	rm -f "$stats_file"
}
trap cleanup EXIT

start_ns=$(date +%s%N)
set +e
/usr/bin/time -q -f '%M' -o "$stats_file" "$@"
status=$?
set -e
end_ns=$(date +%s%N)

case "$start_ns" in
	''|*[!0-9]*) echo "invalid start timestamp" >&2; exit 1 ;;
esac
case "$end_ns" in
	''|*[!0-9]*) echo "invalid end timestamp" >&2; exit 1 ;;
esac
wall_ms=$(( (end_ns - start_ns) / 1000000 ))
if [ "$wall_ms" -lt 1 ]; then
	wall_ms=1
fi
peak_rss_kib=$(tr -d '[:space:]' < "$stats_file")
case "$peak_rss_kib" in
	''|*[!0-9]*) echo "invalid peak RSS measurement for $metric_name: $(cat "$stats_file")" >&2; exit 1 ;;
esac
if [ "$peak_rss_kib" -lt 1 ]; then
	peak_rss_kib=1
fi

printf '%s\t%s\t%s\n' "$metric_name" "$wall_ms" "$peak_rss_kib" >> "$metrics_file"
exit "$status"
