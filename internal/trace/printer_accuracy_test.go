// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

// Tests for Part B: debug command trace accuracy and context improvements.
// Covers PrintExecutionTrace behaviour for nil, empty, and normal traces.

package trace

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// ── PrintExecutionTrace: nil and empty guards ─────────────────────────────────

func TestPrintExecutionTrace_NilTrace_DoesNotPanic(t *testing.T) {
	var buf bytes.Buffer
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintExecutionTrace panicked on nil trace: %v", r)
		}
	}()
	PrintExecutionTrace(nil, PrintOptions{NoColor: true, Output: &buf})
}

func TestPrintExecutionTrace_NilTrace_PrintsErrorMessage(t *testing.T) {
	var buf bytes.Buffer
	PrintExecutionTrace(nil, PrintOptions{NoColor: true, Output: &buf})

	out := buf.String()
	if !strings.Contains(out, "[FAIL]") {
		t.Errorf("nil trace output should contain [FAIL], got:\n%s", out)
	}
	if !strings.Contains(out, "nil") {
		t.Errorf("nil trace output should mention nil, got:\n%s", out)
	}
}

func TestPrintExecutionTrace_EmptyTrace_DoesNotPanic(t *testing.T) {
	tr := NewExecutionTrace("abc123", 0)
	var buf bytes.Buffer
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintExecutionTrace panicked on empty trace: %v", r)
		}
	}()
	PrintExecutionTrace(tr, PrintOptions{NoColor: true, Output: &buf})
}

func TestPrintExecutionTrace_EmptyTrace_PrintsDiagnosticNotSilent(t *testing.T) {
	tr := NewExecutionTrace("deadbeef", 0)
	var buf bytes.Buffer
	PrintExecutionTrace(tr, PrintOptions{NoColor: true, Output: &buf})

	out := buf.String()
	// Must not produce empty output — a user should see something actionable.
	if strings.TrimSpace(out) == "" {
		t.Error("expected non-empty output for empty trace")
	}
	if !strings.Contains(out, "No execution steps") {
		t.Errorf("empty trace output should say 'No execution steps', got:\n%s", out)
	}
	// Must give actionable suggestions.
	if !strings.Contains(out, "simulator") {
		t.Errorf("empty trace output should mention simulator, got:\n%s", out)
	}
}

func TestPrintExecutionTrace_EmptyTrace_ContainsTransactionHash(t *testing.T) {
	txHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	tr := NewExecutionTrace(txHash, 0)
	var buf bytes.Buffer
	PrintExecutionTrace(tr, PrintOptions{NoColor: true, Output: &buf})

	// Should at least print part of the tx hash so the user knows which
	// transaction produced the empty trace.
	out := buf.String()
	if !strings.Contains(out, txHash[:16]) {
		t.Errorf("empty trace output should contain part of tx hash, got:\n%s", out)
	}
}

// ── PrintExecutionTrace: normal traces ───────────────────────────────────────

func TestPrintExecutionTrace_SingleStep_PrintsHeader(t *testing.T) {
	tr := NewExecutionTrace("abc123", 0)
	tr.AddState(ExecutionState{
		Operation: "contract_call",
		Function:  "transfer",
		EventType: EventTypeContractCall,
		Timestamp: time.Now(),
	})

	var buf bytes.Buffer
	PrintExecutionTrace(tr, PrintOptions{NoColor: true, Output: &buf})

	out := buf.String()
	if !strings.Contains(out, "Transaction Execution Trace") {
		t.Errorf("expected header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Steps") {
		t.Errorf("expected step count in output, got:\n%s", out)
	}
}

func TestPrintExecutionTrace_ErrorState_PrintsFailTag(t *testing.T) {
	tr := NewExecutionTrace("abc123", 0)
	tr.AddState(ExecutionState{
		Operation: "contract_call",
		Function:  "transfer",
		Error:     "insufficient balance",
		Timestamp: time.Now(),
	})

	var buf bytes.Buffer
	PrintExecutionTrace(tr, PrintOptions{NoColor: true, Output: &buf})

	out := buf.String()
	if !strings.Contains(out, "[FAIL]") {
		t.Errorf("expected [FAIL] tag for error state, got:\n%s", out)
	}
	if !strings.Contains(out, "insufficient balance") {
		t.Errorf("expected error message in output, got:\n%s", out)
	}
}

func TestPrintExecutionTrace_MultipleSteps_CountCorrect(t *testing.T) {
	tr := NewExecutionTrace("abc123", 0)
	for i := 0; i < 5; i++ {
		tr.AddState(ExecutionState{
			Operation: "contract_call",
			Timestamp: time.Now(),
		})
	}

	var buf bytes.Buffer
	PrintExecutionTrace(tr, PrintOptions{NoColor: true, Output: &buf})

	out := buf.String()
	if !strings.Contains(out, "5") {
		t.Errorf("expected step count 5 in output, got:\n%s", out)
	}
}

// ── PrintExecutionTrace: verbosity filtering ──────────────────────────────────

func TestPrintExecutionTrace_SummaryVerbosity_NoWasmInstructions(t *testing.T) {
	tr := NewExecutionTrace("abc123", 0)
	tr.AddState(ExecutionState{
		Operation:       "call",
		WasmInstruction: "local.get 1",
		Timestamp:       time.Now(),
	})

	var buf bytes.Buffer
	PrintExecutionTrace(tr, PrintOptions{NoColor: true, Output: &buf, Verbosity: VerbositySummary})

	out := buf.String()
	if strings.Contains(out, "local.get 1") {
		t.Errorf("VerbositySummary should hide WASM instructions, got:\n%s", out)
	}
}

func TestPrintExecutionTrace_VerboseVerbosity_ShowsWasmInstructions(t *testing.T) {
	tr := NewExecutionTrace("abc123", 0)
	tr.AddState(ExecutionState{
		Operation:       "call",
		WasmInstruction: "local.get 1",
		Timestamp:       time.Now(),
	})

	var buf bytes.Buffer
	PrintExecutionTrace(tr, PrintOptions{NoColor: true, Output: &buf, Verbosity: VerbosityVerbose})

	// Verbose mode should not strip anything.
	out := buf.String()
	if !strings.Contains(out, "local.get 1") {
		t.Errorf("VerbosityVerbose should show WASM instructions, got:\n%s", out)
	}
}

// ── ValidateExecutionTrace integration with printer ───────────────────────────

func TestValidateExecutionTrace_BeforePrint_EmptyGivesActionableIssues(t *testing.T) {
	tr := NewExecutionTrace("abc", 0)
	issues := ValidateExecutionTrace(tr)

	if len(issues) == 0 {
		t.Fatal("expected at least one issue for empty trace")
	}
	if !strings.Contains(issues[0], "no steps") {
		t.Errorf("issue should say 'no steps', got: %s", issues[0])
	}
}

func TestValidateExecutionTrace_ValidTrace_NoIssues(t *testing.T) {
	tr := NewExecutionTrace("abc123", 0)
	tr.AddState(ExecutionState{
		Operation: "contract_call",
		Timestamp: time.Now(),
	})

	issues := ValidateExecutionTrace(tr)
	if len(issues) != 0 {
		t.Errorf("expected no issues for valid trace, got: %v", issues)
	}
}

// ── Regression: PrintTraceTree with nil root ──────────────────────────────────

func TestPrintTraceTree_NilRoot_DoesNotPanic(t *testing.T) {
	var buf bytes.Buffer
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintTraceTree panicked on nil root: %v", r)
		}
	}()
	PrintTraceTree(nil, PrintOptions{NoColor: true, Output: &buf})
}

// ── Unrecognised event type in trace state ────────────────────────────────────

func TestValidateEventTypeField_UnknownType_HasSimulatorHint(t *testing.T) {
	diag := ValidateEventTypeField("exotic_operation_xyz")
	if !strings.Contains(diag, "simulator") {
		t.Errorf("diagnostic for unknown event type should mention simulator, got: %s", diag)
	}
	if !strings.Contains(diag, "exotic_operation_xyz") {
		t.Errorf("diagnostic should name the offending type, got: %s", diag)
	}
	if !strings.Contains(diag, "trace accuracy") {
		t.Errorf("diagnostic should mention trace accuracy, got: %s", diag)
	}
}

// ── Cost annotation in printer ────────────────────────────────────────────────

func TestPrintExecutionTrace_WithCostAnnotation_IncludesCost(t *testing.T) {
	tr := NewExecutionTrace("abc", 0)
	tr.AddState(ExecutionState{
		Operation: "contract_call",
		Timestamp: time.Now(),
		Cost: &CostAnnotation{
			CPU:         12345,
			MemoryBytes: 512,
		},
	})

	var buf bytes.Buffer
	PrintExecutionTrace(tr, PrintOptions{NoColor: true, Output: &buf})

	out := buf.String()
	if !strings.Contains(out, "Cost") {
		t.Errorf("expected Cost section in output when CostAnnotation is set, got:\n%s", out)
	}
}
