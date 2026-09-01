package main

import "testing"

// This test is the observed counterexample. It fails at generation N and
// passes only after the generated artifact is installed at generation N+1.
func TestCounterexample(t *testing.T) {
	if got, want := CompileToken("gooo"), "GOOO"; got != want {
		t.Fatalf("counterexample: got %q, want %q", got, want)
	}
}
