# gooo-closed-loop-evolution-runner

An executable, fail-closed closed-loop evolution runner for a small compiler
fixture. The implementation is developed on the feature branch and is
validated by GitHub Actions under Go 1.27.

The input repository remains read-only. Generated evidence and temporary next
generations are written only to caller-owned directories. The runner has no
commit, merge, push, or release authority.

The authoritative `.gooo` program declares eight fixed stages and drives the
chain from an observed compiler counterexample through a typed rewrite,
semantic IR synthesis, a temporary next generation, selected test
verification, an independent bootstrap witness, and byte-identical replay.
The fixed vector is `CLOSED, UNKNOWN, REFUTED, UNKNOWN, CLOSED` for one-bug
repair, ambiguous candidates, semantic drift, missing immutable tool evidence,
and replay.

CI downloads the five locked component release assets from immutable public
releases. Missing or mismatched tool evidence stays UNKNOWN. The generated
repair is a real Go file that changes the fixture's behavior; the evidence
preserves baseline failure and evolved/bootstrap pass terminal records.

Evidence v2 adds exact integer inventory line counts for Go and `.gooo` files,
CI wall/RSS measurements for compile, build, test, conformance, and integration,
zero-valued local execution counters, and exact per-case test counters. CI
rejects missing, null, string, and non-integer measurements, and compares the
fixed vector, terminal witnesses, immutable tool observations, generated
artifacts, and replay bytes with the immutable v0.1.0 evidence.
The field contract is recorded in [`contracts/metrics-v2.json`](contracts/metrics-v2.json).

The runner command is:

```text
go run ./cmd/gooo-closed-loop-evolution-runner run \
  --source .gooo/closed-loop-evolution-runner.gooo \
  --contract contracts/denominator-v1.json \
  --tool-lock contracts/immutable-tool-lock-v1.json \
  --candidates-root examples/candidates \
  --fixture-root /caller-owned/fixture \
  --tools-dir /caller-owned/tools \
  --out /caller-owned/output \
  --source-root .
```

See [`docs/protocol-v1.md`](docs/protocol-v1.md) for the exact protocol and
release boundary.
