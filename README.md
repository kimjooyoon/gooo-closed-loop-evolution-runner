# gooo-closed-loop-evolution-runner

An executable, fail-closed closed-loop evolution runner for a small compiler
fixture. The implementation is developed on the feature branch and is
validated by GitHub Actions under Go 1.27.

The input repository remains read-only. Generated evidence and temporary next
generations are written only to caller-owned directories. The runner has no
commit, merge, push, or release authority.

