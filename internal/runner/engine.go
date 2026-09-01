package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

type RunInput struct {
	MetaPath       string
	ContractPath   string
	ToolLockPath   string
	CandidatesRoot string
	FixtureRoot    string
	ToolsDir       string
	OutputRoot     string
	SourceRoot     string
}

type runContext struct {
	Meta            MetaSource
	Contract        Contract
	Lock            ToolLock
	Candidates      map[string]Candidate
	Tools           []ToolObservation
	ToolByID        map[string]ToolObservation
	SourceRoot      string
	FixtureRoot     string
	ToolsDir        string
	OutputRoot      string
	SourceDigest    string
	ContractDigest  string
	ToolchainDigest string
	RunnerDigest    string
	GeneratedFiles  int
	GeneratedBytes  int
	SelectedTests   int
	ExecutedTests   int
	ReusedTests     int
	FailedTests     int
	UnknownTests    int
}

var goTestTiming = regexp.MustCompile(`\([0-9]+\.[0-9]+s\)`)

func Run(input RunInput) (Evidence, error) {
	started := time.Now()
	meta, err := ParseMeta(input.MetaPath)
	if err != nil {
		return Evidence{}, err
	}
	contract, err := LoadContract(input.ContractPath)
	if err != nil {
		return Evidence{}, err
	}
	lock, err := LoadToolLock(input.ToolLockPath)
	if err != nil {
		return Evidence{}, err
	}
	if err := validateDeclarations(meta, contract, lock); err != nil {
		return Evidence{}, err
	}
	sourceRoot, err := filepath.Abs(input.SourceRoot)
	if err != nil {
		return Evidence{}, err
	}
	fixtureRoot, err := filepath.Abs(input.FixtureRoot)
	if err != nil {
		return Evidence{}, err
	}
	outputRoot, err := filepath.Abs(input.OutputRoot)
	if err != nil {
		return Evidence{}, err
	}
	candidatesRoot, err := filepath.Abs(input.CandidatesRoot)
	if err != nil {
		return Evidence{}, err
	}
	toolsDir := input.ToolsDir
	if toolsDir != "" {
		toolsDir, err = filepath.Abs(toolsDir)
		if err != nil {
			return Evidence{}, err
		}
	}
	if err := ensureExternalEmptyDirectory(fixtureRoot, sourceRoot); err != nil {
		return Evidence{}, fmt.Errorf("fixture root: %w", err)
	}
	if err := ensureExternalEmptyDirectory(outputRoot, sourceRoot); err != nil {
		return Evidence{}, fmt.Errorf("output root: %w", err)
	}
	candidates, err := LoadCandidates(candidatesRoot)
	if err != nil {
		return Evidence{}, err
	}
	byID := make(map[string]Candidate, len(candidates))
	for index := range candidates {
		candidate := candidates[index]
		if err := validateCandidate(candidate, meta); err != nil {
			return Evidence{}, err
		}
		relative, relErr := filepath.Rel(sourceRoot, candidate.SourcePath)
		if relErr == nil && !strings.HasPrefix(relative, "..") {
			candidate.SourcePath = filepath.ToSlash(relative)
		}
		byID[candidate.ID] = candidate
	}
	contractData, err := os.ReadFile(input.ContractPath)
	if err != nil {
		return Evidence{}, err
	}
	lockData, err := os.ReadFile(input.ToolLockPath)
	if err != nil {
		return Evidence{}, err
	}
	toolObservations, toolchainDigest := verifyToolInputs(lock, toolsDir)
	runnerDigest, err := DigestValue(map[string]string{
		"runner":    "gooo-closed-loop-evolution-runner",
		"version":   "v1",
		"source":    meta.SourceDigest,
		"contract":  DigestBytes(contractData),
		"tool_lock": DigestBytes(lockData),
	})
	if err != nil {
		return Evidence{}, err
	}
	toolByID := make(map[string]ToolObservation, len(toolObservations))
	for _, observation := range toolObservations {
		toolByID[observation.ID] = observation
	}
	context := runContext{
		Meta: meta, Contract: contract, Lock: lock, Candidates: byID, Tools: toolObservations, ToolByID: toolByID,
		SourceRoot: sourceRoot, FixtureRoot: fixtureRoot, ToolsDir: toolsDir, OutputRoot: outputRoot,
		SourceDigest: meta.SourceDigest, ContractDigest: DigestBytes(contractData), ToolchainDigest: toolchainDigest, RunnerDigest: runnerDigest,
	}
	templateRoot := filepath.Join(sourceRoot, "fixtures", "compiler")
	if _, err := os.Stat(templateRoot); err != nil {
		return Evidence{}, fmt.Errorf("compiler fixture is unavailable: %w", err)
	}
	context.FixtureRoot = templateRoot
	results := make([]CaseResult, 0, len(meta.Cases))
	for _, declaredCase := range meta.Cases {
		caseResult, evalErr := evaluateCase(&context, declaredCase)
		if evalErr != nil {
			return Evidence{}, evalErr
		}
		results = append(results, caseResult)
	}
	inventory, err := measureInventory(sourceRoot)
	if err != nil {
		return Evidence{}, err
	}
	summary := summarize(results)
	wallMS := int(time.Since(started).Milliseconds())
	if wallMS < 1 {
		wallMS = 1
	}
	peak := peakRSSKiB()
	evidence := Evidence{
		Schema: EvidenceSchema, Version: "v1", SourceDigest: context.SourceDigest, ContractDigest: context.ContractDigest,
		ToolchainDigest: context.ToolchainDigest, RunnerDigest: context.RunnerDigest,
		Precedence: append([]string{}, meta.Precedence...), UnknownFields: append([]string{}, meta.UnknownFields...),
		DenominatorID: meta.Denominator.ID, FixedCaseCount: meta.Denominator.CellCount, FixedStageCount: len(meta.Stages),
		Tools: context.Tools, Summary: summary, Cases: results,
		Metrics:         Metrics{WallMS: wallMS, PeakRSSKiB: peak, Inventory: inventory, Generated: GeneratedMetrics{Files: context.GeneratedFiles, Bytes: context.GeneratedBytes}, Tests: TestMetrics{Total: len(meta.Cases), Selected: context.SelectedTests, Executed: context.ExecutedTests, Reused: context.ReusedTests, Failed: context.FailedTests, Unknown: context.UnknownTests}},
		Authority:       Authority{RepositoryWrites: 0, CommitAuthority: 0, MergeAuthority: 0, ReleaseAuthority: 0},
		AtomicAbortRule: meta.AtomicAbort, ArtifactRule: meta.Artifact,
	}
	artifactNames, err := listRelativeFiles(outputRoot)
	if err != nil {
		return Evidence{}, err
	}
	artifactNames = append(artifactNames, "evidence.json", "runner-report.md")
	sort.Strings(artifactNames)
	evidence.ArtifactNames = artifactNames
	evidence.ArtifactCount = len(artifactNames)
	if err := writeEvidence(outputRoot, evidence); err != nil {
		return Evidence{}, err
	}
	if err := writeReport(outputRoot, evidence); err != nil {
		return Evidence{}, err
	}
	return evidence, nil
}

func verifyToolInputs(lock ToolLock, toolsDir string) ([]ToolObservation, string) {
	observations := make([]ToolObservation, 0, len(lock.Tools))
	canonical := make(map[string]map[string]any, len(lock.Tools))
	for _, tool := range lock.Tools {
		observation := ToolObservation{ID: tool.ID, Repository: tool.Repository, Release: tool.Tag, ReleaseID: tool.ReleaseID, Immutable: tool.Immutable, Asset: tool.Asset.Name, ExpectedDigest: tool.Asset.Digest, Reason: "OK"}
		if !tool.Immutable {
			observation.Reason = "LOCK_RELEASE_NOT_IMMUTABLE"
		} else if toolsDir == "" {
			observation.Reason = "IMMUTABLE_TOOL_EVIDENCE_MISSING"
		} else {
			path := filepath.Join(toolsDir, tool.Asset.Name)
			data, err := os.ReadFile(path)
			if err != nil {
				observation.Reason = "IMMUTABLE_TOOL_EVIDENCE_MISSING"
			} else {
				observation.ObservedDigest = DigestBytes(data)
				if observation.ObservedDigest != tool.Asset.Digest {
					observation.Reason = "IMMUTABLE_TOOL_DIGEST_MISMATCH"
				} else {
					observation.Available = true
				}
			}
		}
		if observation.Available {
			observation.Reason = "OK"
		}
		observations = append(observations, observation)
		canonical[tool.ID] = map[string]any{"expected": observation.ExpectedDigest, "observed": observation.ObservedDigest, "immutable": observation.Immutable, "available": observation.Available}
	}
	digest, _ := DigestValue(canonical)
	return observations, digest
}

func evaluateCase(context *runContext, declared CaseDecl) (CaseResult, error) {
	candidates := make([]Candidate, 0, len(declared.CandidateIDs))
	for _, id := range declared.CandidateIDs {
		candidate, ok := context.Candidates[id]
		if !ok {
			return CaseResult{}, fmt.Errorf("case %q references missing candidate %q", declared.ID, id)
		}
		candidates = append(candidates, candidate)
	}
	rule, ok := findRule(context.Meta.Rules, declared.Rule)
	if !ok {
		return CaseResult{}, fmt.Errorf("case %q references missing rule %q", declared.ID, declared.Rule)
	}
	caseRoot := filepath.Join(context.OutputRoot, "cases", safeName(declared.ID))
	if err := os.MkdirAll(caseRoot, 0o755); err != nil {
		return CaseResult{}, err
	}
	result := CaseResult{Ordinal: declared.Ordinal, ID: declared.ID, Kind: declared.Kind, Expected: declared.Expected, CandidateIDs: append([]string{}, declared.CandidateIDs...), CandidateDigests: candidateDigests(candidates), Improvement: ImprovementPair{State: StateUnknown}}
	stages := make([]StageEvidence, len(context.Meta.Stages))
	empty := emptyDigest()
	for index, stage := range context.Meta.Stages {
		stages[index] = StageEvidence{Ordinal: stage.Ordinal, ID: stage.ID, Capability: stage.Capability, InputDigest: empty, OutputDigest: empty, TerminalReason: "NOT_REACHED", NextOperation: stage.NextOperation, WallMS: 0, PeakRSSKiB: 0}
	}
	result.Stages = stages
	fixtureDigest, err := digestTree(context.FixtureRoot)
	if err != nil {
		return CaseResult{}, err
	}
	firstCandidate := candidates[0]
	stageStarted := time.Now()
	baselineRoot := filepath.Join(caseRoot, "baseline")
	if err := copyFixture(context.FixtureRoot, baselineRoot); err != nil {
		return CaseResult{}, err
	}
	baseline := runTest(baselineRoot, firstCandidate.Rewrite.Test)
	result.Baseline = baseline
	baselineDigest, _ := DigestValue(baseline)
	if baseline.Status == "UNAVAILABLE" {
		unknown := unknownAt(context, 0, "RUN_BASELINE_TEST", "BASELINE_TEST_UNAVAILABLE", "TOOL_UNAVAILABLE", "PROVIDE_GO_1_27_TEST_RUNTIME", []string{"go"})
		finishStage(&result.Stages[0], stageStarted, fixtureDigest, baselineDigest, "BASELINE_TEST_UNAVAILABLE")
		blockStages(result.Stages, context.Meta.Stages, 1, baselineDigest, unknown.Reason)
		result.State, result.Decision, result.AtomicAbort, result.Unknown = StateUnknown, unknown.Reason, true, &unknown
		context.UnknownTests++
		return finalizeCase(context, declared, rule, result)
	}
	if baseline.Status != "FAIL" {
		finishStage(&result.Stages[0], stageStarted, fixtureDigest, baselineDigest, "BASELINE_COUNTEREXAMPLE_NOT_REPRODUCED")
		blockStages(result.Stages, context.Meta.Stages, 1, baselineDigest, "BASELINE_COUNTEREXAMPLE_NOT_REPRODUCED")
		result.State, result.Decision, result.AtomicAbort = StateRefuted, "BASELINE_COUNTEREXAMPLE_NOT_REPRODUCED", true
		return finalizeCase(context, declared, rule, result)
	}
	finishStage(&result.Stages[0], stageStarted, fixtureDigest, baselineDigest, "BASELINE_COUNTEREXAMPLE_OBSERVED")

	stageStarted = time.Now()
	tool, toolOK := context.ToolByID[declared.RequiredTool]
	if !toolOK {
		tool = ToolObservation{ID: declared.RequiredTool, ExpectedDigest: "", Reason: "TOOL_LOCK_ENTRY_MISSING"}
	}
	toolDigest, _ := DigestValue(tool)
	if !toolOK || !tool.Available {
		reason := tool.Reason
		unknownClass := "MISSING_IMMUTABLE_TOOL_EVIDENCE"
		next := "FETCH_AND_VERIFY_IMMUTABLE_RELEASE_ASSET"
		if !toolOK {
			reason, unknownClass, next = "TOOL_LOCK_ENTRY_MISSING", "TOOL_CONTRACT_MISMATCH", "ADD_DIGEST_PINNED_TOOL_LOCK_ENTRY"
		}
		unknown := unknownAt(context, 1, "VERIFY_IMMUTABLE_TOOL", reason, unknownClass, next, []string{declared.RequiredTool})
		finishStage(&result.Stages[1], stageStarted, baselineDigest, toolDigest, "IMMUTABLE_TOOL_EVIDENCE_UNAVAILABLE")
		blockStages(result.Stages, context.Meta.Stages, 2, toolDigest, reason)
		result.State, result.Decision, result.AtomicAbort, result.Unknown = StateUnknown, unknown.Reason, true, &unknown
		context.UnknownTests++
		return finalizeCase(context, declared, rule, result)
	}
	finishStage(&result.Stages[1], stageStarted, baselineDigest, toolDigest, "IMMUTABLE_TOOLS_RESOLVED")

	stageStarted = time.Now()
	candidateDigest, _ := DigestValue(candidates)
	finishStage(&result.Stages[2], stageStarted, toolDigest, candidateDigest, "TYPED_REWRITE_LOWERED")

	stageStarted = time.Now()
	if len(candidates) > 1 && equalPriority(candidates) {
		unknown := unknownAt(context, 3, "CHOOSE_UNIQUE_CANDIDATE", "EQUAL_PRIORITY_TYPED_REWRITES", "AMBIGUOUS_CANDIDATES", "COLLECT_DISAMBIGUATING_EVIDENCE", declared.CandidateIDs)
		finishStage(&result.Stages[3], stageStarted, candidateDigest, empty, "AMBIGUOUS_TYPED_REWRITES")
		blockStages(result.Stages, context.Meta.Stages, 4, empty, unknown.Reason)
		result.State, result.Decision, result.AtomicAbort, result.Unknown = StateUnknown, unknown.Reason, true, &unknown
		context.UnknownTests++
		return finalizeCase(context, declared, rule, result)
	}
	chosen := chooseCandidate(candidates)
	ir := SemanticIR{Schema: "gooo/closed-loop-evolution-runner/semantic-ir/v1", CaseID: declared.ID, Counterexample: Counterexample{FixtureDigest: fixtureDigest, Test: chosen.Rewrite.Test, Terminal: baseline}, Candidate: chosen, SourceDigest: context.SourceDigest, ContractDigest: context.ContractDigest, ToolchainDigest: context.ToolchainDigest, RunnerDigest: context.RunnerDigest}
	irData, err := json.MarshalIndent(ir, "", "  ")
	if err != nil {
		return CaseResult{}, err
	}
	irData = append(irData, '\n')
	irPath := filepath.Join(caseRoot, "synthesis", "semantic-ir.json")
	if err := os.MkdirAll(filepath.Dir(irPath), 0o755); err != nil {
		return CaseResult{}, err
	}
	if err := os.WriteFile(irPath, irData, 0o644); err != nil {
		return CaseResult{}, err
	}
	irDigest := DigestBytes(irData)
	finishStage(&result.Stages[3], stageStarted, candidateDigest, irDigest, "REPAIR_IR_SYNTHESIZED")

	stageStarted = time.Now()
	nextRoot := filepath.Join(caseRoot, "generated", "next-generation")
	if err := copyFixture(context.FixtureRoot, nextRoot); err != nil {
		return CaseResult{}, err
	}
	if err := applyRewrite(nextRoot, chosen); err != nil {
		return CaseResult{}, err
	}
	generated, err := snapshotGenerated(nextRoot)
	if err != nil {
		return CaseResult{}, err
	}
	build := runCommand(nextRoot, []string{"go", "build", "./..."}, "NEXT_GENERATION_BUILT")
	result.Build, result.Generated = build, generated
	context.GeneratedFiles += generatedFileCount(generated)
	context.GeneratedBytes += generated.Bytes
	generationDigest, _ := DigestValue(map[string]any{"artifact": generated, "build": build})
	if build.Status != "PASS" {
		finishStage(&result.Stages[4], stageStarted, irDigest, generationDigest, "NEXT_GENERATION_BUILD_FAILED")
		blockStages(result.Stages, context.Meta.Stages, 5, generationDigest, "NEXT_GENERATION_BUILD_FAILED")
		result.State, result.Decision, result.AtomicAbort = StateRefuted, "NEXT_GENERATION_BUILD_FAILED", true
		return finalizeCase(context, declared, rule, result)
	}
	finishStage(&result.Stages[4], stageStarted, irDigest, generationDigest, "NEXT_GENERATION_BUILT")

	stageStarted = time.Now()
	context.SelectedTests++
	evolved := runTest(nextRoot, chosen.Rewrite.Test)
	result.Evolved = evolved
	context.ExecutedTests++
	evolvedDigest, _ := DigestValue(evolved)
	if evolved.Status != "PASS" {
		finishStage(&result.Stages[5], stageStarted, generationDigest, evolvedDigest, "SEMANTIC_DRIFT")
		blockStages(result.Stages, context.Meta.Stages, 6, evolvedDigest, "SEMANTIC_DRIFT")
		result.State, result.Decision, result.AtomicAbort = StateRefuted, "SEMANTIC_DRIFT", true
		context.FailedTests++
		return finalizeCase(context, declared, rule, result)
	}
	finishStage(&result.Stages[5], stageStarted, generationDigest, evolvedDigest, "SELECTED_TESTS_PASS")

	stageStarted = time.Now()
	bootstrapRoot := filepath.Join(caseRoot, "bootstrap")
	if err := copyFixture(nextRoot, bootstrapRoot); err != nil {
		return CaseResult{}, err
	}
	if err := validateGeneratedGo(filepath.Join(bootstrapRoot, "compiler")); err != nil {
		finishStage(&result.Stages[6], stageStarted, generationDigest, empty, "INDEPENDENT_BOOTSTRAP_PARSE_FAILED")
		blockStages(result.Stages, context.Meta.Stages, 7, empty, "INDEPENDENT_BOOTSTRAP_PARSE_FAILED")
		result.State, result.Decision, result.AtomicAbort = StateRefuted, "INDEPENDENT_BOOTSTRAP_PARSE_FAILED", true
		return finalizeCase(context, declared, rule, result)
	}
	bootstrap := runTest(bootstrapRoot, chosen.Rewrite.Test)
	result.Bootstrap = bootstrap
	context.ExecutedTests++
	bootstrapDigest, _ := DigestValue(bootstrap)
	bootstrapOutputDigest, _ := DigestValue(map[string]string{"artifact": generated.Digest, "test": bootstrap.StableDigest})
	if bootstrap.Status != "PASS" || bootstrap.StableDigest != evolved.StableDigest {
		finishStage(&result.Stages[6], stageStarted, generationDigest, bootstrapOutputDigest, "INDEPENDENT_BOOTSTRAP_DRIFT")
		blockStages(result.Stages, context.Meta.Stages, 7, bootstrapOutputDigest, "INDEPENDENT_BOOTSTRAP_DRIFT")
		result.State, result.Decision, result.AtomicAbort = StateRefuted, "INDEPENDENT_BOOTSTRAP_DRIFT", true
		return finalizeCase(context, declared, rule, result)
	}
	finishStage(&result.Stages[6], stageStarted, generationDigest, bootstrapOutputDigest, "INDEPENDENT_BOOTSTRAP_PASS")

	stageStarted = time.Now()
	if declared.Replay {
		replayRoot := filepath.Join(caseRoot, "generated", "replay")
		if err := copyFixture(context.FixtureRoot, replayRoot); err != nil {
			return CaseResult{}, err
		}
		if err := applyRewrite(replayRoot, chosen); err != nil {
			return CaseResult{}, err
		}
		replayGenerated, snapshotErr := snapshotGenerated(replayRoot)
		if snapshotErr != nil {
			return CaseResult{}, snapshotErr
		}
		replayTest := runTest(replayRoot, chosen.Rewrite.Test)
		result.ReplayGenerated = replayGenerated
		context.GeneratedFiles += generatedFileCount(replayGenerated)
		context.GeneratedBytes += replayGenerated.Bytes
		context.ReusedTests++
		context.ExecutedTests++
		result.ReplayEqual = sameArtifact(generated, replayGenerated) && sameTerminal(evolved, replayTest)
		replayDigest, _ := DigestValue(map[string]any{"artifact": replayGenerated, "test": replayTest})
		if !result.ReplayEqual {
			finishStage(&result.Stages[7], stageStarted, bootstrapOutputDigest, replayDigest, "BYTE_IDENTICAL_REPLAY_MISMATCH")
			result.State, result.Decision, result.AtomicAbort = StateRefuted, "BYTE_IDENTICAL_REPLAY_MISMATCH", true
			return finalizeCase(context, declared, rule, result)
		}
		finishStage(&result.Stages[7], stageStarted, bootstrapOutputDigest, replayDigest, "BYTE_IDENTICAL_REPLAY")
	} else {
		finishStage(&result.Stages[7], stageStarted, bootstrapOutputDigest, generated.Digest, "CLOSED_GENERATION_FINALIZED")
	}
	result.State, result.Decision, result.AtomicAbort, result.PromotedArtifact = StateClosed, rule.Terminal, false, true
	result.Improvement = ImprovementPair{State: StateClosed, ScenarioDigest: scenarioDigest(declared, chosen), SourceDigest: context.SourceDigest, ContractDigest: context.ContractDigest, ToolchainDigest: context.ToolchainDigest, RunnerDigest: context.RunnerDigest, Before: baseline, After: evolved, BeforeArtifact: emptyArtifactSnapshot(), AfterArtifact: generated}
	return finalizeCase(context, declared, rule, result)
}

func finalizeCase(context *runContext, declared CaseDecl, rule RuleDecl, result CaseResult) (CaseResult, error) {
	if result.State == StateUnknown {
		result.PromotedArtifact = false
		result.Improvement.State = StateUnknown
	}
	if result.State == StateRefuted {
		result.PromotedArtifact = false
		result.Improvement.State = StateRefuted
	}
	if result.State != rule.Outcome {
		return CaseResult{}, fmt.Errorf("case %q evaluated as %s; declared rule %q requires %s", declared.ID, result.State, rule.ID, rule.Outcome)
	}
	return result, nil
}

func unknownAt(context *runContext, stage int, step, reason, unknownClass, next string, blockedBy []string) Unknown {
	stageID := "unknown"
	if stage >= 0 && stage < len(context.Meta.Stages) {
		stageID = context.Meta.Stages[stage].ID
	}
	return Unknown{Stage: stageID, Step: step, Reason: reason, UnknownClass: unknownClass, NextOperation: next, BlockedBy: append([]string{}, blockedBy...)}
}

func finishStage(stage *StageEvidence, started time.Time, inputDigest, outputDigest, terminal string) {
	stage.InputDigest, stage.OutputDigest, stage.TerminalReason = inputDigest, outputDigest, terminal
	stage.WallMS = int(time.Since(started).Milliseconds())
	if stage.WallMS < 1 {
		stage.WallMS = 1
	}
	stage.PeakRSSKiB = peakRSSKiB()
}

func blockStages(stages []StageEvidence, declarations []StageDecl, from int, inputDigest, reason string) {
	for index := from; index < len(stages); index++ {
		stages[index].InputDigest = inputDigest
		stages[index].OutputDigest = emptyDigest()
		stages[index].TerminalReason = "BLOCKED_BY_" + reason
		stages[index].NextOperation = declarations[index].NextOperation
	}
}

func findRule(rules []RuleDecl, id string) (RuleDecl, bool) {
	for _, rule := range rules {
		if rule.ID == id {
			return rule, true
		}
	}
	return RuleDecl{}, false
}

func candidateDigests(candidates []Candidate) []string {
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, candidate.SourceDigest)
	}
	return result
}

func equalPriority(candidates []Candidate) bool {
	if len(candidates) < 2 {
		return false
	}
	for _, candidate := range candidates[1:] {
		if candidate.Priority != candidates[0].Priority {
			return false
		}
	}
	return true
}

func chooseCandidate(candidates []Candidate) Candidate {
	chosen := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Priority < chosen.Priority || (candidate.Priority == chosen.Priority && candidate.ID < chosen.ID) {
			chosen = candidate
		}
	}
	return chosen
}

func scenarioDigest(declared CaseDecl, candidate Candidate) string {
	digest, _ := DigestValue(map[string]any{"case": declared.ID, "kind": declared.Kind, "candidate": candidate.SourceDigest, "test": candidate.Rewrite.Test})
	return digest
}

func emptyDigest() string {
	digest, _ := DigestValue(map[string]string{})
	return digest
}

func emptyArtifactSnapshot() ArtifactSnapshot {
	return ArtifactSnapshot{Files: stringMap{}, Digest: emptyDigest(), Bytes: 0}
}

func applyRewrite(root string, candidate Candidate) error {
	if candidate.Operation.Kind != "install_normalizer" {
		return fmt.Errorf("unsupported rewrite operation %q", candidate.Operation.Kind)
	}
	if candidate.Operation.Artifact != candidate.Rewrite.Output {
		return fmt.Errorf("rewrite output and operation artifact differ")
	}
	if !strings.HasPrefix(filepath.ToSlash(candidate.Operation.Artifact), "compiler/generated/") {
		return fmt.Errorf("rewrite artifact escapes compiler generated root")
	}
	var body string
	switch candidate.Operation.Value {
	case "upper":
		body = "package main\n\nfunc init() { normalizationMode = \"upper\" }\n"
	case "lower":
		body = "package main\n\nfunc init() { normalizationMode = \"lower\" }\n"
	default:
		return fmt.Errorf("unknown normalizer value %q", candidate.Operation.Value)
	}
	path := filepath.Join(root, filepath.FromSlash(candidate.Operation.Artifact))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

func runTest(root, testName string) TerminalEvidence {
	return runCommand(root, []string{"go", "test", "-count=1", "-run", "^" + testName + "$", "./..."}, "")
}

func runCommand(root string, command []string, terminalReason string) TerminalEvidence {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	evidence := TerminalEvidence{Command: append([]string{}, command...), ExitCode: -1, Status: "UNAVAILABLE", Stdout: "", Stderr: "", TerminalReason: terminalReason}
	err := cmd.Run()
	evidence.Stdout, evidence.Stderr = stdout.String(), stderr.String()
	if err == nil {
		evidence.ExitCode, evidence.Status = 0, "PASS"
	} else if exitError, ok := err.(*exec.ExitError); ok {
		evidence.ExitCode, evidence.Status = exitError.ExitCode(), "FAIL"
	}
	raw, _ := json.Marshal(map[string]string{"stdout": evidence.Stdout, "stderr": evidence.Stderr})
	evidence.OutputDigest = DigestBytes(raw)
	stable, _ := json.Marshal(map[string]any{"status": evidence.Status, "exit_code": evidence.ExitCode, "stdout": stableTestOutput(evidence.Stdout), "stderr": stableTestOutput(evidence.Stderr)})
	evidence.StableDigest = DigestBytes(stable)
	return evidence
}

func stableTestOutput(value string) string {
	return goTestTiming.ReplaceAllString(value, "(<duration>)")
}

func sameTerminal(left, right TerminalEvidence) bool {
	return left.Status == right.Status && left.ExitCode == right.ExitCode && left.StableDigest == right.StableDigest
}

func sameArtifact(left, right ArtifactSnapshot) bool {
	if left.Digest != right.Digest || left.Bytes != right.Bytes || len(left.Files) != len(right.Files) {
		return false
	}
	for path, digest := range left.Files {
		if right.Files[path] != digest {
			return false
		}
	}
	return true
}

func snapshotGenerated(root string) (ArtifactSnapshot, error) {
	result := emptyArtifactSnapshot()
	generatedRoot := filepath.Join(root, "compiler", "generated")
	files := stringMap{}
	err := filepath.WalkDir(generatedRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = DigestBytes(data)
		result.Bytes += len(data)
		return nil
	})
	if os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		return ArtifactSnapshot{}, err
	}
	result.Files = files
	result.Digest, err = DigestValue(files)
	return result, err
}

func generatedFileCount(snapshot ArtifactSnapshot) int { return len(snapshot.Files) }

func digestTree(root string) (string, error) {
	files := stringMap{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = DigestBytes(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	return DigestValue(files)
}

func copyFixture(source, destination string) error {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func validateGeneratedGo(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		if _, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors); err != nil {
			return fmt.Errorf("generated Go parse failed for %q: %w", path, err)
		}
		return nil
	})
}

func measureInventory(root string) (Inventory, error) {
	result := Inventory{RootReadmeExcluded: true}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == ".git" || strings.HasPrefix(relative, ".git"+string(filepath.Separator)) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if relative != "." {
				result.DescendantDirs++
			}
			return nil
		}
		if relative == "README.md" {
			return nil
		}
		result.RegularFiles++
		extension := filepath.Ext(path)
		if extension == ".go" {
			result.GoFiles++
		}
		if extension == ".gooo" {
			result.GoooFiles++
		}
		if extension == ".go" || extension == ".gooo" {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			result.PhysicalLines += physicalLines(data)
		}
		return nil
	})
	return result, err
}

func physicalLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := 1
	for _, value := range data {
		if value == '\n' {
			lines++
		}
	}
	if data[len(data)-1] == '\n' {
		lines--
	}
	return lines
}

func listRelativeFiles(root string) ([]string, error) {
	result := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result = append(result, filepath.ToSlash(relative))
		return nil
	})
	sort.Strings(result)
	return result, err
}

func peakRSSKiB() int {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	value := int(stats.Sys / 1024)
	if value < 1 {
		return 1
	}
	return value
}

func ensureExternalEmptyDirectory(path, sourceRoot string) error {
	if isWithin(sourceRoot, path) {
		return fmt.Errorf("path %q is inside source root %q", path, sourceRoot)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("path %q must be empty", path)
	}
	return nil
}

func isWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return true
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func summarize(results []CaseResult) Summary {
	result := Summary{Generated: len(results)}
	for _, item := range results {
		switch item.State {
		case StateClosed:
			result.Closed++
		case StateUnknown:
			result.Unknown++
		case StateRefuted:
			result.Refuted++
		}
	}
	return result
}

func writeEvidence(root string, evidence Evidence) error {
	data, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(root, "evidence.json"), data, 0o644)
}

func writeReport(root string, evidence Evidence) error {
	var builder strings.Builder
	builder.WriteString("# Closed-loop evolution runner evidence\n\n")
	fmt.Fprintf(&builder, "Precedence: `%s`\n\n", strings.Join(evidence.Precedence, " > "))
	fmt.Fprintf(&builder, "Fixed vector: `%s`\n\n", fixedVector(evidence.Cases))
	builder.WriteString("| case | expected | state | promoted | replay_equal | decision |\n|---|---|---|---|---|---|\n")
	for _, item := range evidence.Cases {
		fmt.Fprintf(&builder, "| `%s` | `%s` | `%s` | %t | %t | `%s` |\n", item.ID, item.Expected, item.State, item.PromotedArtifact, item.ReplayEqual, item.Decision)
	}
	builder.WriteString("\n## Stage evidence\n\n")
	for _, item := range evidence.Cases {
		fmt.Fprintf(&builder, "### %s\n\n", item.ID)
		for _, stage := range item.Stages {
			fmt.Fprintf(&builder, "- `%02d %s`: input `%s`, output `%s`, capability `%s`, terminal `%s`, next `%s`, wall_ms `%d`, peak_rss_kib `%d`\n", stage.Ordinal, stage.ID, stage.InputDigest, stage.OutputDigest, stage.Capability, stage.TerminalReason, stage.NextOperation, stage.WallMS, stage.PeakRSSKiB)
		}
	}
	builder.WriteString("\n## Exact integer metrics and authority\n\n")
	fmt.Fprintf(&builder, "- inventory Go/Gooo/physical_lines: `%d`/`%d`/`%d`; descendant_dirs: `%d`; regular_files: `%d`\n", evidence.Metrics.Inventory.GoFiles, evidence.Metrics.Inventory.GoooFiles, evidence.Metrics.Inventory.PhysicalLines, evidence.Metrics.Inventory.DescendantDirs, evidence.Metrics.Inventory.RegularFiles)
	fmt.Fprintf(&builder, "- generated files/bytes: `%d`/`%d`; wall_ms: `%d`; peak_rss_kib: `%d`\n", evidence.Metrics.Generated.Files, evidence.Metrics.Generated.Bytes, evidence.Metrics.WallMS, evidence.Metrics.PeakRSSKiB)
	fmt.Fprintf(&builder, "- tests total/selected/executed/reused/failed/unknown: `%d`/`%d`/`%d`/`%d`/`%d`/`%d`\n", evidence.Metrics.Tests.Total, evidence.Metrics.Tests.Selected, evidence.Metrics.Tests.Executed, evidence.Metrics.Tests.Reused, evidence.Metrics.Tests.Failed, evidence.Metrics.Tests.Unknown)
	fmt.Fprintf(&builder, "- repository_writes: `%d`; commit_authority: `%d`; merge_authority: `%d`; release_authority: `%d`\n", evidence.Authority.RepositoryWrites, evidence.Authority.CommitAuthority, evidence.Authority.MergeAuthority, evidence.Authority.ReleaseAuthority)
	builder.WriteString("- root README.md is excluded from inventory; all generated output is caller-owned.\n")
	return os.WriteFile(filepath.Join(root, "runner-report.md"), []byte(builder.String()), 0o644)
}

func fixedVector(cases []CaseResult) string {
	parts := make([]string, 0, len(cases))
	for _, item := range cases {
		parts = append(parts, item.ID+"="+item.State)
	}
	return strings.Join(parts, ",")
}
