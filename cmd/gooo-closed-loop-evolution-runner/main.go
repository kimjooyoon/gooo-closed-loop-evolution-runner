package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/gooo-closed-loop-evolution-runner/internal/runner"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "run" {
		fmt.Fprintln(os.Stderr, "usage: gooo-closed-loop-evolution-runner run --source PATH --contract PATH --tool-lock PATH --candidates-root PATH --fixture-root PATH --tools-dir PATH --out PATH --source-root PATH")
		os.Exit(2)
	}
	flags := flag.NewFlagSet("run", flag.ExitOnError)
	metaPath := flags.String("source", ".gooo/closed-loop-evolution-runner.gooo", "authoritative Gooo metacode")
	contractPath := flags.String("contract", "contracts/denominator-v1.json", "fixed denominator contract")
	toolLockPath := flags.String("tool-lock", "contracts/immutable-tool-lock-v1.json", "immutable release lock")
	candidatesRoot := flags.String("candidates-root", "examples/candidates", "typed rewrite candidates")
	fixtureRoot := flags.String("fixture-root", "", "empty caller-owned temporary directory")
	toolsDir := flags.String("tools-dir", "", "caller-owned directory containing verified release assets")
	outputRoot := flags.String("out", "", "empty caller-owned output directory")
	sourceRoot := flags.String("source-root", ".", "input repository root")
	if err := flags.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if *fixtureRoot == "" || *outputRoot == "" {
		fmt.Fprintln(os.Stderr, "--fixture-root and --out are required caller-owned directories")
		os.Exit(2)
	}
	evidence, err := runner.Run(runner.RunInput{MetaPath: *metaPath, ContractPath: *contractPath, ToolLockPath: *toolLockPath, CandidatesRoot: *candidatesRoot, FixtureRoot: *fixtureRoot, ToolsDir: *toolsDir, OutputRoot: *outputRoot, SourceRoot: *sourceRoot})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("vector=%s closed=%d unknown=%d refuted=%d generated_files=%d generated_bytes=%d\n", vector(evidence), evidence.Summary.Closed, evidence.Summary.Unknown, evidence.Summary.Refuted, evidence.Metrics.Generated.Files, evidence.Metrics.Generated.Bytes)
}

func vector(evidence runner.Evidence) string {
	parts := make([]string, 0, len(evidence.Cases))
	for _, item := range evidence.Cases {
		parts = append(parts, item.ID+"="+item.State)
	}
	return fmt.Sprintf("%v", parts)
}
