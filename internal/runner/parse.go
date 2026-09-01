package runner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func ParseMeta(path string) (MetaSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MetaSource{}, err
	}
	meta := MetaSource{SourceDigest: DigestBytes(data)}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	line := 0
	for scanner.Scan() {
		line++
		fields := splitFields(scanner.Text())
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		switch fields[0] {
		case "gooo":
			if len(fields) != 3 || fields[1] != "closed_loop_evolution_runner" {
				return MetaSource{}, fmt.Errorf("line %d: invalid gooo header", line)
			}
			meta.Schema, meta.Version = MetaSchema, fields[2]
		case "namespace":
			if len(fields) != 2 {
				return MetaSource{}, fmt.Errorf("line %d: invalid namespace", line)
			}
			meta.Namespace = fields[1]
		case "effect":
			if len(fields) != 2 {
				return MetaSource{}, fmt.Errorf("line %d: invalid effect", line)
			}
			meta.Effects = append(meta.Effects, fields[1])
		case "capability":
			if len(fields) < 3 {
				return MetaSource{}, fmt.Errorf("line %d: capability requires effects", line)
			}
			meta.Capabilities = append(meta.Capabilities, CapabilityDecl{Name: fields[1], Effects: append([]string{}, fields[2:]...)})
		case "precedence":
			if len(fields) != 2 {
				return MetaSource{}, fmt.Errorf("line %d: invalid precedence", line)
			}
			meta.Precedence = strings.Split(fields[1], ">")
		case "unknown_fields":
			if len(fields) != 2 {
				return MetaSource{}, fmt.Errorf("line %d: invalid unknown_fields", line)
			}
			meta.UnknownFields = strings.Split(fields[1], ",")
		case "forbidden_effects":
			if len(fields) < 2 {
				return MetaSource{}, fmt.Errorf("line %d: forbidden effect set is empty", line)
			}
			meta.Forbidden = append(meta.Forbidden, fields[1:]...)
		case "stage":
			values, parseErr := keyValues(fields[1:])
			if parseErr != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", line, parseErr)
			}
			ordinal, parseErr := integer(values, "ordinal")
			if parseErr != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", line, parseErr)
			}
			meta.Stages = append(meta.Stages, StageDecl{Ordinal: ordinal, ID: values["id"], Capability: values["capability"], Terminal: values["terminal"], NextOperation: values["next_operation"]})
		case "rule":
			values, parseErr := keyValues(fields[1:])
			if parseErr != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", line, parseErr)
			}
			meta.Rules = append(meta.Rules, RuleDecl{ID: values["id"], Condition: values["condition"], Outcome: values["outcome"], Terminal: values["terminal"]})
		case "denominator":
			values, parseErr := keyValues(fields[1:])
			if parseErr != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", line, parseErr)
			}
			count, parseErr := integer(values, "cell_count")
			if parseErr != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", line, parseErr)
			}
			meta.Denominator = DenominatorDecl{ID: values["id"], CellCount: count}
		case "case":
			values, parseErr := keyValues(fields[1:])
			if parseErr != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", line, parseErr)
			}
			ordinal, parseErr := integer(values, "ordinal")
			if parseErr != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", line, parseErr)
			}
			meta.Cases = append(meta.Cases, CaseDecl{Ordinal: ordinal, ID: values["id"], Kind: values["kind"], Rule: values["rule"], Expected: values["expected"], CandidateIDs: splitList(values["candidates"]), RequiredTool: values["required_tool"], Replay: values["replay"] == "true"})
		case "atomic_abort":
			values, parseErr := keyValues(fields[1:])
			if parseErr != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", line, parseErr)
			}
			meta.AtomicAbort = AtomicAbortDecl{States: splitList(values["states"]), PromoteArtifact: values["promote_artifact"] == "true", PartialPromotion: values["partial_promotion"] == "true"}
		case "artifact":
			values, parseErr := keyValues(fields[1:])
			if parseErr != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", line, parseErr)
			}
			meta.Artifact = ArtifactDecl{ClosedOnly: values["closed_only"] == "true", Path: values["path"], Digest: values["digest"]}
		default:
			return MetaSource{}, fmt.Errorf("line %d: unknown declaration %q", line, fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return MetaSource{}, err
	}
	return meta, nil
}

func ParseCandidate(path string) (Candidate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Candidate{}, err
	}
	candidate := Candidate{Schema: CandidateSchema, Version: "v1", SourcePath: filepath.ToSlash(path), SourceDigest: DigestBytes(data)}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	line := 0
	for scanner.Scan() {
		line++
		fields := splitFields(scanner.Text())
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		switch fields[0] {
		case "gooo":
			if len(fields) != 3 || fields[1] != "rewrite_candidate" || fields[2] != "v1" {
				return Candidate{}, fmt.Errorf("%s:%d: invalid candidate header", path, line)
			}
		case "candidate":
			values, parseErr := keyValues(fields[1:])
			if parseErr != nil {
				return Candidate{}, fmt.Errorf("%s:%d: %w", path, line, parseErr)
			}
			candidate.ID, candidate.Type = values["id"], values["type"]
			candidate.Priority, _ = optionalInteger(values, "priority")
		case "origin":
			values, parseErr := keyValues(fields[1:])
			if parseErr != nil {
				return Candidate{}, fmt.Errorf("%s:%d: %w", path, line, parseErr)
			}
			candidate.Origin = Origin{Author: values["author"], Source: values["source"]}
		case "capability":
			candidate.Capabilities = append(candidate.Capabilities, fields[1:]...)
		case "effect":
			values, parseErr := keyValues(fields[1:])
			if parseErr != nil {
				return Candidate{}, fmt.Errorf("%s:%d: %w", path, line, parseErr)
			}
			candidate.EffectPre, candidate.EffectPost = splitList(values["pre"]), splitList(values["post"])
		case "footprint":
			values, parseErr := keyValues(fields[1:])
			if parseErr != nil {
				return Candidate{}, fmt.Errorf("%s:%d: %w", path, line, parseErr)
			}
			candidate.ReadFootprint, candidate.WriteFootprint = splitList(values["read"]), splitList(values["write"])
		case "rewrite":
			values, parseErr := keyValues(fields[1:])
			if parseErr != nil {
				return Candidate{}, fmt.Errorf("%s:%d: %w", path, line, parseErr)
			}
			candidate.Rewrite = Rewrite{Input: values["input"], Output: values["output"], Test: values["test"]}
		case "operation":
			values, parseErr := keyValues(fields[1:])
			if parseErr != nil {
				return Candidate{}, fmt.Errorf("%s:%d: %w", path, line, parseErr)
			}
			candidate.Operation = Operation{Kind: values["kind"], Artifact: values["artifact"], Value: values["value"]}
		case "terminal":
			values, parseErr := keyValues(fields[1:])
			if parseErr != nil {
				return Candidate{}, fmt.Errorf("%s:%d: %w", path, line, parseErr)
			}
			candidate.Terminal = TerminalContract{Baseline: values["baseline"], Evolved: values["evolved"]}
		default:
			return Candidate{}, fmt.Errorf("%s:%d: unknown declaration %q", path, line, fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return Candidate{}, err
	}
	return candidate, nil
}

func LoadCandidates(root string) ([]Candidate, error) {
	paths := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && filepath.Ext(path) == ".gooo" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	result := make([]Candidate, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		candidate, parseErr := ParseCandidate(path)
		if parseErr != nil {
			return nil, parseErr
		}
		if seen[candidate.ID] {
			return nil, fmt.Errorf("duplicate candidate id %q", candidate.ID)
		}
		seen[candidate.ID] = true
		result = append(result, candidate)
	}
	return result, nil
}

func LoadContract(path string) (Contract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Contract{}, err
	}
	var contract Contract
	if err := json.Unmarshal(data, &contract); err != nil {
		return Contract{}, fmt.Errorf("decode contract: %w", err)
	}
	return contract, nil
}

func LoadToolLock(path string) (ToolLock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ToolLock{}, err
	}
	var lock ToolLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return ToolLock{}, fmt.Errorf("decode tool lock: %w", err)
	}
	return lock, nil
}

func validateDeclarations(meta MetaSource, contract Contract, lock ToolLock) error {
	if meta.Schema != MetaSchema || meta.Version != "v1" || meta.Namespace != "gooo://closed-loop-evolution-runner/v1" {
		return fmt.Errorf("meta source identity mismatch")
	}
	if contract.Schema != ContractSchema || contract.Version != "v1" || !contract.Fixed || contract.ID != meta.Denominator.ID || contract.CellCount != meta.Denominator.CellCount || contract.CellCount != FixedCaseCount {
		return fmt.Errorf("fixed denominator declaration mismatch")
	}
	if lock.Schema != ToolLockSchema || lock.Version != "v1" || lock.Authority != "immutable_public_release_only" || !lock.ImmutableReleaseRequired || lock.SiblingCheckoutConsumption {
		return fmt.Errorf("immutable tool lock boundary mismatch")
	}
	if !sameStrings(meta.Precedence, []string{StateRefuted, StateUnknown, StateClosed}) {
		return fmt.Errorf("resolution precedence must be REFUTED>UNKNOWN>CLOSED")
	}
	if !sameStrings(meta.UnknownFields, []string{"stage", "step", "reason", "unknown_class", "next_operation", "blocked_by"}) {
		return fmt.Errorf("UNKNOWN six-field contract mismatch")
	}
	if len(meta.Stages) != FixedStageCount {
		return fmt.Errorf("expected exactly %d fixed stages", FixedStageCount)
	}
	seenStages := map[string]bool{}
	for index, stage := range meta.Stages {
		if stage.Ordinal != index+1 || stage.ID == "" || stage.Capability == "" || stage.Terminal == "" || stage.NextOperation == "" || seenStages[stage.ID] {
			return fmt.Errorf("invalid fixed stage %d", index+1)
		}
		seenStages[stage.ID] = true
	}
	if len(meta.Rules) != FixedCaseCount || len(meta.Cases) != FixedCaseCount || len(contract.Cases) != FixedCaseCount {
		return fmt.Errorf("fixed semantic denominator cardinality mismatch")
	}
	seenRules := map[string]bool{}
	for _, rule := range meta.Rules {
		if rule.ID == "" || rule.Condition == "" || rule.Outcome == "" || rule.Terminal == "" || seenRules[rule.ID] {
			return fmt.Errorf("invalid semantic rule %q", rule.ID)
		}
		seenRules[rule.ID] = true
	}
	for index := range meta.Cases {
		metaCase, contractCase := meta.Cases[index], contract.Cases[index]
		if metaCase.Ordinal != index+1 || metaCase.ID == "" || metaCase.Rule == "" || len(metaCase.CandidateIDs) == 0 || metaCase.RequiredTool == "" || metaCase.ID != contractCase.ID || metaCase.Kind != contractCase.Kind || metaCase.Expected != contractCase.Expected || contractCase.Ordinal != index+1 {
			return fmt.Errorf("fixed case %d does not match the declared contract", index+1)
		}
	}
	if !sameStrings(meta.AtomicAbort.States, []string{StateUnknown, StateRefuted}) || meta.AtomicAbort.PromoteArtifact || meta.AtomicAbort.PartialPromotion {
		return fmt.Errorf("atomic abort declaration must deny UNKNOWN/REFUTED promotion")
	}
	if !meta.Artifact.ClosedOnly || meta.Artifact.Path == "" || meta.Artifact.Digest != "sha256" {
		return fmt.Errorf("artifact declaration must be closed-only with sha256 digest")
	}
	if len(lock.Tools) != 5 {
		return fmt.Errorf("immutable tool denominator must contain five tools")
	}
	return nil
}

func validateCandidate(candidate Candidate, meta MetaSource) error {
	if candidate.Schema != CandidateSchema || candidate.Version != "v1" || candidate.ID == "" || candidate.Type == "" || candidate.Priority < 1 {
		return fmt.Errorf("candidate %q has invalid typed identity", candidate.ID)
	}
	if candidate.Origin.Author == "" || candidate.Origin.Source == "" || len(candidate.Capabilities) == 0 || len(candidate.EffectPre) == 0 || len(candidate.EffectPost) == 0 || len(candidate.ReadFootprint) == 0 || len(candidate.WriteFootprint) == 0 || candidate.Rewrite.Input == "" || candidate.Rewrite.Output == "" || candidate.Rewrite.Test == "" || candidate.Operation.Kind == "" || candidate.Operation.Artifact == "" || candidate.Operation.Value == "" || candidate.Terminal.Baseline == "" || candidate.Terminal.Evolved == "" {
		return fmt.Errorf("candidate %q is missing typed rewrite fields", candidate.ID)
	}
	if !containsAll(meta.Effects, append(append([]string{}, candidate.EffectPre...), candidate.EffectPost...)) {
		return fmt.Errorf("candidate %q uses an undeclared effect", candidate.ID)
	}
	if !contains(meta.Effects, "READ_IMMUTABLE_TOOL") {
		return fmt.Errorf("meta source lacks immutable tool effect")
	}
	for _, capability := range candidate.Capabilities {
		if !capabilityDeclared(meta, capability) {
			return fmt.Errorf("candidate %q uses undeclared capability %q", candidate.ID, capability)
		}
	}
	if !strings.HasPrefix(filepath.ToSlash(candidate.Operation.Artifact), "compiler/generated/") || candidate.Operation.Kind != "install_normalizer" {
		return fmt.Errorf("candidate %q is outside the typed normalizer rewrite", candidate.ID)
	}
	return nil
}

func capabilityDeclared(meta MetaSource, wanted string) bool {
	for _, capability := range meta.Capabilities {
		if capability.Name == wanted {
			return true
		}
	}
	return false
}

func splitFields(line string) []string {
	var result []string
	var builder strings.Builder
	inQuote := false
	for _, character := range strings.TrimSpace(line) {
		switch character {
		case '"':
			inQuote = !inQuote
			builder.WriteRune(character)
		case ' ', '\t':
			if inQuote {
				builder.WriteRune(character)
			} else if builder.Len() > 0 {
				result = append(result, builder.String())
				builder.Reset()
			}
		default:
			builder.WriteRune(character)
		}
	}
	if builder.Len() > 0 {
		result = append(result, builder.String())
	}
	return result
}

func keyValues(fields []string) (map[string]string, error) {
	values := make(map[string]string, len(fields))
	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, fmt.Errorf("invalid key/value %q", field)
		}
		values[parts[0]] = strings.Trim(parts[1], "\"")
	}
	return values, nil
}

func integer(values map[string]string, key string) (int, error) {
	value, ok := values[key]
	if !ok || value == "" {
		return 0, fmt.Errorf("missing %s", key)
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q", key, value)
	}
	return number, nil
}

func optionalInteger(values map[string]string, key string) (int, error) {
	value := values[key]
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func splitList(value string) []string {
	if value == "" {
		return nil
	}
	var result []string
	for _, part := range strings.FieldsFunc(value, func(character rune) bool { return character == ',' || character == '+' }) {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func containsAll(haystack, needles []string) bool {
	for _, needle := range needles {
		if !contains(haystack, needle) {
			return false
		}
	}
	return true
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
