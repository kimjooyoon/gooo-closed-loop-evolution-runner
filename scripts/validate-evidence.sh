#!/usr/bin/env bash
set -euo pipefail

evidence=${1:?evidence JSON path is required}
test -f "$evidence"

jq -e '
  def exact_integer:
    if type == "number" then floor == . else false end;
  def nonnegative_integer:
    if exact_integer then . >= 0 else false end;
  def positive_integer:
    if exact_integer then . > 0 else false end;
  .schema == "gooo/closed-loop-evolution-runner/evidence/v2" and
  (.metrics.inventory.go_physical_lines | positive_integer) and
  (.metrics.inventory.gooo_physical_lines | positive_integer) and
  (.metrics.compile_wall_ms | positive_integer) and
  (.metrics.compile_peak_rss_kib | positive_integer) and
  (.metrics.build_wall_ms | positive_integer) and
  (.metrics.build_peak_rss_kib | positive_integer) and
  (.metrics.test_wall_ms | positive_integer) and
  (.metrics.test_peak_rss_kib | positive_integer) and
  (.metrics.conformance_wall_ms | positive_integer) and
  (.metrics.conformance_peak_rss_kib | positive_integer) and
  (.metrics.integration_wall_ms | positive_integer) and
  (.metrics.integration_peak_rss_kib | positive_integer) and
  (.metrics.local_test_executions | exact_integer) and (.metrics.local_test_executions == 0) and
  (.metrics.local_build_executions | exact_integer) and (.metrics.local_build_executions == 0) and
  (.metrics.local_vet_executions | exact_integer) and (.metrics.local_vet_executions == 0) and
  (.metrics.local_conformance_executions | exact_integer) and (.metrics.local_conformance_executions == 0) and
  (.metrics.local_integration_executions | exact_integer) and (.metrics.local_integration_executions == 0) and
  ([.cases[] | [.tests_total, .tests_selected, .tests_executed, .tests_reused, .tests_failed, .tests_unknown] | all(.[]; nonnegative_integer)] | all)
' "$evidence" >/dev/null

printf 'evidence measurement contract: exact integer fields and zero local executions verified\n'
