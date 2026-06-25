// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"strings"
)

// TraceInputError is returned when one or more trace-related CLI inputs are
// invalid. Each element in Failures is an actionable description of a single
// problem, so users can fix all issues in one pass.
type TraceInputError struct {
	Failures []string
}

func (e *TraceInputError) Error() string {
	if len(e.Failures) == 1 {
		return e.Failures[0]
	}
	lines := make([]string, 0, len(e.Failures)+1)
	lines = append(lines, fmt.Sprintf("%d trace input validation error(s):", len(e.Failures)))
	for i, f := range e.Failures {
		lines = append(lines, fmt.Sprintf("  %d. %s", i+1, f))
	}
	return strings.Join(lines, "\n")
}

// ValidateTraceInputs checks trace-related CLI flags for validity before any
// simulation or network fetch occurs.
//
// Parameters:
//   - verbosity: value of --trace-verbosity (may be empty → default normal)
//   - exportFormat: value of --format (may be empty → default text)
//   - eventFilter: value of an event-type filter (may be empty → no filter)
//   - outputPath: path supplied to --trace-output (may be empty → no export)
//
// Returns nil when all inputs are valid, or a *TraceInputError listing every
// problem found.
func ValidateTraceInputs(verbosity, exportFormat, eventFilter, outputPath string) error {
	var failures []string

	// Verbosity.
	if verbosity != "" {
		if _, err := ParseVerbosity(verbosity); err != nil {
			failures = append(failures, fmt.Sprintf(
				"invalid --trace-verbosity %q — must be one of: summary, normal, verbose",
				verbosity,
			))
		}
	}

	// Export format.
	if exportFormat != "" {
		switch strings.ToLower(strings.TrimSpace(exportFormat)) {
		case "text", "json", "html", "markdown", "md":
			// valid
		default:
			failures = append(failures, fmt.Sprintf(
				"invalid trace export format %q — must be one of: text, json, html, markdown",
				exportFormat,
			))
		}
	}

	// Event filter.
	if eventFilter != "" {
		valid := false
		for _, t := range AllFilterableEventTypes() {
			if strings.EqualFold(eventFilter, t) {
				valid = true
				break
			}
		}
		if !valid {
			failures = append(failures, fmt.Sprintf(
				"invalid event filter %q — must be one of: %s",
				eventFilter,
				strings.Join(AllFilterableEventTypes(), ", "),
			))
		}
	}

	// Output path sanity: must not be a bare directory path.
	if outputPath != "" && (strings.HasSuffix(outputPath, "/") || strings.HasSuffix(outputPath, "\\")) {
		failures = append(failures, fmt.Sprintf(
			"--trace-output %q looks like a directory path; provide a full file path (e.g. ./trace.html)",
			outputPath,
		))
	}

	if len(failures) > 0 {
		return &TraceInputError{Failures: failures}
	}
	return nil
}

// ValidateEventTypeField checks whether an explicitly supplied EventType value
// in an ExecutionState is a known, supported value. Unknown values are
// normalised to EventTypeOther by ClassifyEventType — calling this function
// allows callers to surface a warning when the simulator emits an unrecognised
// event type rather than silently discarding it.
//
// Returns a non-empty diagnostic string when the value is unrecognised.
func ValidateEventTypeField(eventType string) string {
	if eventType == "" {
		return "" // empty is fine; the type will be inferred
	}
	normalised := normalizeEventType(eventType)
	if normalised == EventTypeOther {
		return fmt.Sprintf(
			"unrecognised event type %q (normalised to %q); "+
				"expected one of: %s. Trace accuracy may be reduced for this step. "+
				"Check that your simulator version is compatible with this version of Glassbox",
			eventType,
			EventTypeOther,
			strings.Join(append(AllFilterableEventTypes(), EventTypeOther), ", "),
		)
	}
	return ""
}

// ValidateExecutionTrace checks an ExecutionTrace for structural correctness
// and returns a list of diagnostic messages (non-fatal unless otherwise noted).
//
// Checks:
//   - Trace is not nil.
//   - States slice is not empty (empty trace → diagnostic warning).
//   - Each state has a non-negative Step that matches its slice index.
//   - Unrecognised EventType fields are noted with their step index.
//
// This is deliberately permissive: it returns all issues at once so callers can
// choose whether to abort or merely warn.
func ValidateExecutionTrace(t *ExecutionTrace) []string {
	if t == nil {
		return []string{"execution trace is nil"}
	}

	var issues []string

	if len(t.States) == 0 {
		issues = append(issues, fmt.Sprintf(
			"execution trace for transaction %q contains no steps — "+
				"the simulator did not produce any diagnostic events. "+
				"Check that the transaction envelope is valid and the simulator binary is up-to-date",
			truncateForDiag(t.TransactionHash),
		))
		return issues // nothing further to check on an empty trace
	}

	// Per-step checks.
	for i, state := range t.States {
		if state.Step != i {
			issues = append(issues, fmt.Sprintf(
				"step index mismatch at position %d: state.Step=%d "+
					"(trace may have been modified after construction; trace accuracy may be affected)",
				i, state.Step,
			))
		}
		if diag := ValidateEventTypeField(state.EventType); diag != "" {
			issues = append(issues, fmt.Sprintf("step %d: %s", i, diag))
		}
	}

	return issues
}

// truncateForDiag trims a string for use in diagnostic messages.
func truncateForDiag(s string) string {
	if len(s) > 20 {
		return s[:17] + "..."
	}
	return s
}
