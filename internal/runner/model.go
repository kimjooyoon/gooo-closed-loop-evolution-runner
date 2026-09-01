package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const (
	MetaSchema      = "gooo/closed-loop-evolution-runner/source/v1"
	ContractSchema  = "gooo/closed-loop-evolution-runner/denominator/v1"
	EvidenceSchema  = "gooo/closed-loop-evolution-runner/evidence/v2"
	CandidateSchema = "gooo/closed-loop-evolution-runner/candidate/v1"
	ToolLockSchema  = "gooo/closed-loop-evolution-runner/immutable-tool-lock/v1"
	StateClosed     = "CLOSED"
	StateUnknown    = "UNKNOWN"
	StateRefuted    = "REFUTED"
	FixedCaseCount  = 5
	FixedStageCount = 8
)

type MetaSource struct {
	Schema        string
	Version       string
	Namespace     string
	Effects       []string
	Capabilities  []CapabilityDecl
	Precedence    []string
	UnknownFields []string
	Forbidden     []string
	Stages        []StageDecl
	Rules         []RuleDecl
	Denominator   DenominatorDecl
	Cases         []CaseDecl
	AtomicAbort   AtomicAbortDecl
	Artifact      ArtifactDecl
	Measurement   MeasurementDecl
	SourceDigest  string
}

type CapabilityDecl struct {
	Name    string
	Effects []string
}

type StageDecl struct {
	Ordinal       int
	ID            string
	Capability    string
	Terminal      string
	NextOperation string
}

type RuleDecl struct {
	ID        string
	Condition string
	Outcome   string
	Terminal  string
}

type DenominatorDecl struct {
	ID        string
	CellCount int
}

type CaseDecl struct {
	Ordinal      int
	ID           string
	Kind         string
	Rule         string
	Expected     string
	CandidateIDs []string
	RequiredTool string
	Replay       bool
}

type AtomicAbortDecl struct {
	States           []string
	PromoteArtifact  bool
	PartialPromotion bool
}

type ArtifactDecl struct {
	ClosedOnly bool
	Path       string
	Digest     string
}

type MeasurementDecl struct {
	Schema               string
	Version              string
	IntegerFields        []string
	LocalExecutionFields []string
	CaseFields           []string
	RootReadmeExcluded   bool
}

type Contract struct {
	Schema    string         `json:"schema"`
	ID        string         `json:"id"`
	Version   string         `json:"version"`
	CellCount int            `json:"cell_count"`
	Fixed     bool           `json:"fixed"`
	Cases     []ContractCase `json:"cases"`
}

type ContractCase struct {
	Ordinal  int    `json:"ordinal"`
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Expected string `json:"expected"`
}

type ToolLock struct {
	Schema                     string     `json:"schema"`
	Version                    string     `json:"version"`
	Authority                  string     `json:"authority"`
	ImmutableReleaseRequired   bool       `json:"immutable_release_required"`
	SiblingCheckoutConsumption bool       `json:"sibling_checkout_consumption"`
	Tools                      []ToolDecl `json:"tools"`
}

type ToolDecl struct {
	ID           string    `json:"id"`
	Repository   string    `json:"repository"`
	Tag          string    `json:"tag"`
	TagObject    string    `json:"tag_object"`
	TargetCommit string    `json:"target_commit"`
	ReleaseID    int       `json:"release_id"`
	Immutable    bool      `json:"immutable"`
	Asset        ToolAsset `json:"asset"`
}

type ToolAsset struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
	Role   string `json:"role"`
}

type ToolObservation struct {
	ID             string `json:"id"`
	Repository     string `json:"repository"`
	Release        string `json:"release"`
	ReleaseID      int    `json:"release_id"`
	Immutable      bool   `json:"immutable"`
	Asset          string `json:"asset"`
	ExpectedDigest string `json:"expected_digest"`
	ObservedDigest string `json:"observed_digest"`
	Available      bool   `json:"available"`
	Reason         string `json:"reason"`
}

type Candidate struct {
	Schema         string           `json:"schema"`
	Version        string           `json:"version"`
	ID             string           `json:"id"`
	Type           string           `json:"type"`
	Priority       int              `json:"priority"`
	Origin         Origin           `json:"origin"`
	Capabilities   []string         `json:"capabilities"`
	EffectPre      []string         `json:"effect_pre"`
	EffectPost     []string         `json:"effect_post"`
	ReadFootprint  []string         `json:"read_footprint"`
	WriteFootprint []string         `json:"write_footprint"`
	Rewrite        Rewrite          `json:"rewrite"`
	Operation      Operation        `json:"operation"`
	Terminal       TerminalContract `json:"terminal"`
	SourcePath     string           `json:"source_path"`
	SourceDigest   string           `json:"source_digest"`
}

type Origin struct {
	Author string `json:"author"`
	Source string `json:"source"`
}

type Rewrite struct {
	Input  string `json:"input"`
	Output string `json:"output"`
	Test   string `json:"test"`
}

type Operation struct {
	Kind     string `json:"kind"`
	Artifact string `json:"artifact"`
	Value    string `json:"value"`
}

type TerminalContract struct {
	Baseline string `json:"baseline"`
	Evolved  string `json:"evolved"`
}

type Counterexample struct {
	FixtureDigest string           `json:"fixture_digest"`
	Test          string           `json:"test"`
	Terminal      TerminalEvidence `json:"terminal"`
}

type SemanticIR struct {
	Schema          string         `json:"schema"`
	CaseID          string         `json:"case_id"`
	Counterexample  Counterexample `json:"counterexample"`
	Candidate       Candidate      `json:"candidate"`
	SourceDigest    string         `json:"source_digest"`
	ContractDigest  string         `json:"contract_digest"`
	ToolchainDigest string         `json:"toolchain_digest"`
	RunnerDigest    string         `json:"runner_digest"`
}

type TerminalEvidence struct {
	Command        []string `json:"command"`
	Status         string   `json:"status"`
	ExitCode       int      `json:"exit_code"`
	Stdout         string   `json:"stdout"`
	Stderr         string   `json:"stderr"`
	OutputDigest   string   `json:"output_digest"`
	StableDigest   string   `json:"stable_digest"`
	TerminalReason string   `json:"terminal_reason"`
}

type ArtifactSnapshot struct {
	Files  stringMap `json:"files"`
	Digest string    `json:"digest"`
	Bytes  int       `json:"bytes"`
}

type stringMap map[string]string

type Unknown struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type StageEvidence struct {
	Ordinal        int    `json:"ordinal"`
	ID             string `json:"id"`
	Capability     string `json:"capability"`
	InputDigest    string `json:"input_digest"`
	OutputDigest   string `json:"output_digest"`
	TerminalReason string `json:"terminal_reason"`
	NextOperation  string `json:"next_operation"`
	WallMS         int    `json:"wall_ms"`
	PeakRSSKiB     int    `json:"peak_rss_kib"`
}

type ImprovementPair struct {
	State           string           `json:"state"`
	ScenarioDigest  string           `json:"scenario_digest"`
	SourceDigest    string           `json:"source_digest"`
	ContractDigest  string           `json:"contract_digest"`
	ToolchainDigest string           `json:"toolchain_digest"`
	RunnerDigest    string           `json:"runner_digest"`
	Before          TerminalEvidence `json:"before"`
	After           TerminalEvidence `json:"after"`
	BeforeArtifact  ArtifactSnapshot `json:"before_artifact"`
	AfterArtifact   ArtifactSnapshot `json:"after_artifact"`
}

type CaseResult struct {
	Ordinal          int              `json:"ordinal"`
	ID               string           `json:"id"`
	Kind             string           `json:"kind"`
	Expected         string           `json:"expected"`
	State            string           `json:"state"`
	Decision         string           `json:"decision"`
	CandidateIDs     []string         `json:"candidate_ids"`
	CandidateDigests []string         `json:"candidate_digests"`
	Stages           []StageEvidence  `json:"stages"`
	Baseline         TerminalEvidence `json:"baseline"`
	Build            TerminalEvidence `json:"build"`
	Evolved          TerminalEvidence `json:"evolved"`
	Bootstrap        TerminalEvidence `json:"bootstrap"`
	Generated        ArtifactSnapshot `json:"generated"`
	ReplayGenerated  ArtifactSnapshot `json:"replay_generated"`
	ReplayEqual      bool             `json:"replay_equal"`
	Improvement      ImprovementPair  `json:"improvement"`
	AtomicAbort      bool             `json:"atomic_abort"`
	PromotedArtifact bool             `json:"promoted_artifact"`
	TestsTotal       int              `json:"tests_total"`
	TestsSelected    int              `json:"tests_selected"`
	TestsExecuted    int              `json:"tests_executed"`
	TestsReused      int              `json:"tests_reused"`
	TestsFailed      int              `json:"tests_failed"`
	TestsUnknown     int              `json:"tests_unknown"`
	Unknown          *Unknown         `json:"unknown,omitempty"`
}

type Summary struct {
	Generated int `json:"generated"`
	Closed    int `json:"closed"`
	Unknown   int `json:"unknown"`
	Refuted   int `json:"refuted"`
}

type Inventory struct {
	GoFiles            int  `json:"go_files"`
	GoooFiles          int  `json:"gooo_files"`
	GoPhysicalLines    int  `json:"go_physical_lines"`
	GoooPhysicalLines  int  `json:"gooo_physical_lines"`
	PhysicalLines      int  `json:"physical_lines"`
	DescendantDirs     int  `json:"descendant_dirs"`
	RegularFiles       int  `json:"regular_files"`
	RootReadmeExcluded bool `json:"root_readme_excluded"`
}

type GeneratedMetrics struct {
	Files int `json:"files"`
	Bytes int `json:"bytes"`
}

type TestMetrics struct {
	Total    int `json:"total"`
	Selected int `json:"selected"`
	Executed int `json:"executed"`
	Reused   int `json:"reused"`
	Failed   int `json:"failed"`
	Unknown  int `json:"unknown"`
}

type Metrics struct {
	WallMS                     int              `json:"wall_ms"`
	PeakRSSKiB                 int              `json:"peak_rss_kib"`
	Inventory                  Inventory        `json:"inventory"`
	Generated                  GeneratedMetrics `json:"generated"`
	Tests                      TestMetrics      `json:"tests"`
	CompileWallMS              int              `json:"compile_wall_ms"`
	CompilePeakRSSKiB          int              `json:"compile_peak_rss_kib"`
	BuildWallMS                int              `json:"build_wall_ms"`
	BuildPeakRSSKiB            int              `json:"build_peak_rss_kib"`
	TestWallMS                 int              `json:"test_wall_ms"`
	TestPeakRSSKiB             int              `json:"test_peak_rss_kib"`
	ConformanceWallMS          int              `json:"conformance_wall_ms"`
	ConformancePeakRSSKiB      int              `json:"conformance_peak_rss_kib"`
	IntegrationWallMS          int              `json:"integration_wall_ms"`
	IntegrationPeakRSSKiB      int              `json:"integration_peak_rss_kib"`
	LocalTestExecutions        int              `json:"local_test_executions"`
	LocalBuildExecutions       int              `json:"local_build_executions"`
	LocalVetExecutions         int              `json:"local_vet_executions"`
	LocalConformanceExecutions int              `json:"local_conformance_executions"`
	LocalIntegrationExecutions int              `json:"local_integration_executions"`
}

type Authority struct {
	RepositoryWrites int `json:"repository_writes"`
	CommitAuthority  int `json:"commit_authority"`
	MergeAuthority   int `json:"merge_authority"`
	ReleaseAuthority int `json:"release_authority"`
}

type Evidence struct {
	Schema          string            `json:"schema"`
	Version         string            `json:"version"`
	SourceDigest    string            `json:"source_digest"`
	ContractDigest  string            `json:"contract_digest"`
	ToolchainDigest string            `json:"toolchain_digest"`
	RunnerDigest    string            `json:"runner_digest"`
	Precedence      []string          `json:"precedence"`
	UnknownFields   []string          `json:"unknown_fields"`
	DenominatorID   string            `json:"denominator_id"`
	FixedCaseCount  int               `json:"fixed_case_count"`
	FixedStageCount int               `json:"fixed_stage_count"`
	Tools           []ToolObservation `json:"tools"`
	Summary         Summary           `json:"summary"`
	Cases           []CaseResult      `json:"cases"`
	Metrics         Metrics           `json:"metrics"`
	Authority       Authority         `json:"authority"`
	ArtifactNames   []string          `json:"artifact_names"`
	ArtifactCount   int               `json:"artifact_count"`
	AtomicAbortRule AtomicAbortDecl   `json:"atomic_abort_rule"`
	ArtifactRule    ArtifactDecl      `json:"artifact_rule"`
}

func DigestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func DigestValue(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return DigestBytes(data), nil
}
