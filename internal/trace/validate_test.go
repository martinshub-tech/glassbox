// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"strings"
	"testing"
	"time"
)

// ── ValidateTraceInputs ───────────────────────────────────────────────────────

func TestValidateTraceInputs_AllValid(t *testing.T) {
	if err := ValidateTraceInputs("normal", "text", "", ""); err != nil {
		t.Errorf("expected nil for valid inputs, got: %v", err)
	}
	if err := ValidateTraceInputs("summary", "json", "trap", "out.json"); err != nil {
		t.Errorf("expected nil for valid inputs, got: %v", err)
	}
	if err := ValidateTraceInputs("verbose", "html", "contract_call", "report.html"); err != nil {
		t.Errorf("expected nil for valid inputs, got: %v", err)
	}
	if err := ValidateTraceInputs("", "", "", ""); err != nil {
		t.Errorf("expected nil for all-empty inputs, got: %v", err)
	}
}

func TestValidateTraceInputs_InvalidVerbosity(t *testing.T) {
	err := ValidateTraceInputs("ultra", "", "", "")
	if err == nil {
		t.Fatal("expected error for invalid verbosity")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ultra") {
		t.Errorf("error should include the bad value, got: %s", msg)
	}
	if !strings.Contains(msg, "summary") {
		t.Errorf("error should list valid options, got: %s", msg)
	}
}

func TestValidateTraceInputs_InvalidExportFormat(t *testing.T) {
	err := ValidateTraceInputs("", "xml", "", "")
	if err == nil {
		t.Fatal("expected error for invalid export format")
	}
	msg := err.Error()
	if !strings.Contains(msg, "xml") {
		t.Errorf("error should include the bad value, got: %s", msg)
	}
	if !strings.Contains(msg, "html") {
		t.Errorf("error should list valid options, got: %s", msg)
	}
}

func TestValidateTraceInputs_InvalidEventFilter(t *testing.T) {
	err := ValidateTraceInputs("", "", "unknown_event", "")
	if err == nil {
		t.Fatal("expected error for invalid event filter")
	}
	msg := err.Error()
	if !strings.Contains(msg, "unknown_event") {
		t.Errorf("error should include the bad filter, got: %s", msg)
	}
	if !strings.Contains(msg, "trap") {
		t.Errorf("error should list valid event types, got: %s", msg)
	}
}

func TestValidateTraceInputs_MultipleFailures(t *testing.T) {
	err := ValidateTraceInputs("extreme", "xml", "badtype", "")
	if err == nil {
		t.Fatal("expected error for multiple invalid inputs")
	}
	msg := err.Error()
	if !strings.Contains(msg, "3") {
		t.Errorf("expected 3 failures in error, got: %s", msg)
	}
	// Each bad value should appear.
	for _, bad := range []string{"extreme", "xml", "badtype"} {
		if !strings.Contains(msg, bad) {
			t.Errorf("error should mention %q, got: %s", bad, msg)
		}
	}
}

func TestValidateTraceInputs_ValidEventFilters(t *testing.T) {
	for _, f := range AllFilterableEventTypes() {
		if err := ValidateTraceInputs("", "", f, ""); err != nil {
			t.Errorf("ValidateTraceInputs with filter %q returned unexpected error: %v", f, err)
		}
	}
}

func TestValidateTraceInputs_DirectoryOutputPath(t *testing.T) {
	err := ValidateTraceInputs("", "", "", "/some/dir/")
	if err == nil {
		t.Error("expected error for directory-looking output path")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("expected 'directory' in error, got: %v", err)
	}
}

// ── ValidateEventTypeField ────────────────────────────────────────────────────

func TestValidateEventTypeField_Empty(t *testing.T) {
	if diag := ValidateEventTypeField(""); diag != "" {
		t.Errorf("expected empty diagnostic for empty event type, got: %s", diag)
	}
}

func TestValidateEventTypeField_KnownTypes(t *testing.T) {
	known := []string{
		EventTypeTrap, EventTypeContractCall, EventTypeHostFunction, EventTypeAuth,
		"traps", "contract call", "contractcall", "host function", "host_fn",
		"auth_event",
	}
	for _, k := range known {
		if diag := ValidateEventTypeField(k); diag != "" {
			t.Errorf("expected no diagnostic for known type %q, got: %s", k, diag)
		}
	}
}

func TestValidateEventTypeField_UnknownType(t *testing.T) {
	diag := ValidateEventTypeField("mysterious_event")
	if diag == "" {
		t.Error("expected diagnostic for unknown event type")
	}
	if !strings.Contains(diag, "mysterious_event") {
		t.Errorf("diagnostic should name the unknown type, got: %s", diag)
	}
	if !strings.Contains(diag, "trace accuracy") {
		t.Errorf("diagnostic should mention trace accuracy, got: %s", diag)
	}
	if !strings.Contains(diag, "simulator") {
		t.Errorf("diagnostic should suggest checking simulator version, got: %s", diag)
	}
}

// ── ValidateExecutionTrace ────────────────────────────────────────────────────

func TestValidateExecutionTrace_Nil(t *testing.T) {
	issues := ValidateExecutionTrace(nil)
	if len(issues) == 0 {
		t.Error("expected issues for nil trace")
	}
	if !strings.Contains(issues[0], "nil") {
		t.Errorf("expected nil mention in issue, got: %s", issues[0])
	}
}

func TestValidateExecutionTrace_Empty(t *testing.T) {
	tr := NewExecutionTrace("abc123", 0)
	issues := ValidateExecutionTrace(tr)
	if len(issues) == 0 {
		t.Error("expected issues for empty trace")
	}
	msg := issues[0]
	if !strings.Contains(msg, "no steps") {
		t.Errorf("expected 'no steps' in issue, got: %s", msg)
	}
	// Should give actionable context.
	if !strings.Contains(msg, "simulator") {
		t.Errorf("expected simulator mention in issue, got: %s", msg)
	}
}

func TestValidateExecutionTrace_Valid(t *testing.T) {
	tr := NewExecutionTrace("abc123", 0)
	tr.AddState(ExecutionState{
		Operation: "contract_call",
		EventType: EventTypeContractCall,
	})
	tr.AddState(ExecutionState{
		Operation: "auth",
		EventType: EventTypeAuth,
	})

	issues := ValidateExecutionTrace(tr)
	if len(issues) != 0 {
		t.Errorf("expected no issues for valid trace, got: %v", issues)
	}
}

func TestValidateExecutionTrace_StepIndexMismatch(t *testing.T) {
	tr := NewExecutionTrace("abc123", 0)
	tr.AddState(ExecutionState{Operation: "call"})
	tr.AddState(ExecutionState{Operation: "auth"})

	// Manually corrupt the step index of one state.
	tr.States[1].Step = 99

	issues := ValidateExecutionTrace(tr)
	found := false
	for _, iss := range issues {
		if strings.Contains(iss, "step index mismatch") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected step index mismatch issue, got: %v", issues)
	}
}

func TestValidateExecutionTrace_UnrecognisedEventType(t *testing.T) {
	tr := NewExecutionTrace("abc123", 0)
	tr.AddState(ExecutionState{
		Operation: "mystery_op",
		EventType: "completely_unknown_event",
	})

	issues := ValidateExecutionTrace(tr)
	if len(issues) == 0 {
		t.Error("expected issue for unrecognised event type")
	}
	found := false
	for _, iss := range issues {
		if strings.Contains(iss, "completely_unknown_event") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unrecognised event type in issues, got: %v", issues)
	}
}

func TestValidateExecutionTrace_EmptyEventType_NoIssue(t *testing.T) {
	// States with empty EventType are fine — the type is inferred at runtime.
	tr := NewExecutionTrace("abc123", 0)
	tr.AddState(ExecutionState{Operation: "contract_call"})

	issues := ValidateExecutionTrace(tr)
	if len(issues) != 0 {
		t.Errorf("expected no issues for empty EventType, got: %v", issues)
	}
}

// ── TraceInputError ───────────────────────────────────────────────────────────

func TestTraceInputError_SingleFailure(t *testing.T) {
	e := &TraceInputError{Failures: []string{"single problem"}}
	if e.Error() != "single problem" {
		t.Errorf("single failure should not be wrapped, got: %s", e.Error())
	}
}

func TestTraceInputError_MultipleFailures(t *testing.T) {
	e := &TraceInputError{Failures: []string{"problem one", "problem two", "problem three"}}
	msg := e.Error()
	if !strings.Contains(msg, "3") {
		t.Errorf("should mention count 3, got: %s", msg)
	}
	if !strings.Contains(msg, "problem one") {
		t.Errorf("should include first failure, got: %s", msg)
	}
	if !strings.Contains(msg, "problem three") {
		t.Errorf("should include last failure, got: %s", msg)
	}
	// Should be numbered.
	if !strings.Contains(msg, "1.") {
		t.Errorf("should number failures starting from 1, got: %s", msg)
	}
}

// ── AddState preserves step index for ValidateExecutionTrace ─────────────────

func TestAddState_StepIndicesAreConsistent(t *testing.T) {
	tr := NewExecutionTrace("tx", 0)
	for i := 0; i < 5; i++ {
		tr.AddState(ExecutionState{Operation: "step"})
	}
	issues := ValidateExecutionTrace(tr)
	if len(issues) != 0 {
		t.Errorf("expected no issues after AddState, got: %v", issues)
	}
}

// ── Regression: truncateForDiag ───────────────────────────────────────────────

func TestTruncateForDiag(t *testing.T) {
	short := truncateForDiag("abc")
	if short != "abc" {
		t.Errorf("short string should be unchanged, got: %s", short)
	}
	long := truncateForDiag(strings.Repeat("x", 64))
	if len(long) > 20 {
		t.Errorf("truncated string should be ≤20 chars, got length %d: %s", len(long), long)
	}
	if !strings.HasSuffix(long, "...") {
		t.Errorf("truncated string should end with '...', got: %s", long)
	}
}

// ── Filter validation cases ───────────────────────────────────────────────────

func TestValidateTraceInputs_ValidMarkdownFormat(t *testing.T) {
	for _, f := range []string{"markdown", "md", "html", "text", "json"} {
		if err := ValidateTraceInputs("", f, "", ""); err != nil {
			t.Errorf("format %q should be valid, got: %v", f, err)
		}
	}
}

// ── Integration: ValidateExecutionTrace + AddState with known event types ─────

func TestValidateExecutionTrace_MixedKnownUnknown(t *testing.T) {
	tr := NewExecutionTrace("txhash", 0)
	tr.AddState(ExecutionState{Operation: "contract_call", EventType: EventTypeContractCall})
	tr.AddState(ExecutionState{Operation: "host_fn", EventType: "alien_type"})
	tr.AddState(ExecutionState{Operation: "auth", EventType: EventTypeAuth})

	issues := ValidateExecutionTrace(tr)

	// Exactly one issue: the alien_type at step 1.
	if len(issues) != 1 {
		t.Errorf("expected 1 issue for one bad event type, got %d: %v", len(issues), issues)
	}
	if !strings.Contains(issues[0], "alien_type") {
		t.Errorf("issue should name 'alien_type', got: %s", issues[0])
	}
	if !strings.Contains(issues[0], "step 1") {
		t.Errorf("issue should name step index, got: %s", issues[0])
	}
}

// ── Edge: ValidateTraceInputs does not reject AuthEventType aliases ───────────

func TestValidateTraceInputs_AuthEventAlias(t *testing.T) {
	// The filter must be one of AllFilterableEventTypes(), so "auth" is valid.
	if err := ValidateTraceInputs("", "", EventTypeAuth, ""); err != nil {
		t.Errorf("auth filter should be valid: %v", err)
	}
}

// Ensure NewExecutionTrace returns a trace that passes ValidateExecutionTrace
// without any states (even though it will report no-steps warning).
func TestNewExecutionTrace_EmptyIsConsistent(t *testing.T) {
	tr := NewExecutionTrace("abc", 10)
	if tr == nil {
		t.Fatal("NewExecutionTrace returned nil")
	}
	issues := ValidateExecutionTrace(tr)
	// One expected issue: empty trace.
	if len(issues) != 1 {
		t.Errorf("expected exactly 1 issue (empty trace), got %d: %v", len(issues), issues)
	}
}

// Ensure validate trace with large traces doesn't panic.
func TestValidateExecutionTrace_LargeTrace(t *testing.T) {
	tr := NewExecutionTrace("largetx", 100)
	for i := 0; i < 500; i++ {
		tr.AddState(ExecutionState{
			Operation: "contract_call",
			Timestamp: time.Now(),
		})
	}
	issues := ValidateExecutionTrace(tr)
	if len(issues) != 0 {
		t.Errorf("expected no issues for large valid trace, got: %v", issues)
	}
}
