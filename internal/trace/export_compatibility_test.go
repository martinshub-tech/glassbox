// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTraceFormatVersion_String(t *testing.T) {
	v := TraceFormatVersion{Major: 1, Minor: 2, Patch: 3}
	if got := v.String(); got != "1.2.3" {
		t.Errorf("String() = %q, want %q", got, "1.2.3")
	}
}

func TestTraceFormatVersion_IsCompatibleWith(t *testing.T) {
	tests := []struct {
		name     string
		v        TraceFormatVersion
		other    TraceFormatVersion
		want     bool
	}{
		{
			name:  "identical versions are compatible",
			v:     TraceFormatVersion{1, 0, 0},
			other: TraceFormatVersion{1, 0, 0},
			want:  true,
		},
		{
			name:  "newer minor is compatible (can read older)",
			v:     TraceFormatVersion{1, 2, 0},
			other: TraceFormatVersion{1, 1, 0},
			want:  true,
		},
		{
			name:  "older minor is not compatible (cannot read newer minor)",
			v:     TraceFormatVersion{1, 0, 0},
			other: TraceFormatVersion{1, 1, 0},
			want:  false,
		},
		{
			name:  "different major is never compatible",
			v:     TraceFormatVersion{2, 0, 0},
			other: TraceFormatVersion{1, 0, 0},
			want:  false,
		},
		{
			name:  "different major both ways not compatible",
			v:     TraceFormatVersion{1, 0, 0},
			other: TraceFormatVersion{2, 0, 0},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.IsCompatibleWith(tt.other); got != tt.want {
				t.Errorf("IsCompatibleWith() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExportVersionedTrace(t *testing.T) {
	trace := NewExecutionTrace("versioned-test", 10)
	trace.AddState(ExecutionState{
		Operation: "test",
		Timestamp: time.Now(),
	})

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "versioned.json")

	opts := ExportOptions{}
	compatOpts := DefaultCompatibilityOptions()

	err := ExportVersionedTrace(trace, "json", outputPath, opts, compatOpts)
	if err != nil {
		t.Fatalf("ExportVersionedTrace failed: %v", err)
	}

	// Verify the file contains version info
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}

	var versioned VersionedTrace
	if err := json.Unmarshal(data, &versioned); err != nil {
		t.Fatalf("failed to parse versioned trace: %v", err)
	}

	if versioned.Version.Major != CurrentFormatVersion.Major {
		t.Errorf("version major mismatch: got %d, want %d", versioned.Version.Major, CurrentFormatVersion.Major)
	}

	if versioned.Trace == nil {
		t.Fatal("trace is nil in versioned export")
	}

	if versioned.Trace.TransactionHash != trace.TransactionHash {
		t.Errorf("transaction hash mismatch")
	}
}

func TestLoadVersionedTrace(t *testing.T) {
	t.Run("load versioned trace", func(t *testing.T) {
		trace := NewExecutionTrace("load-test", 10)
		trace.AddState(ExecutionState{Operation: "test", Timestamp: time.Now()})

		tmpDir := t.TempDir()
		outputPath := filepath.Join(tmpDir, "load-test.json")

		// Export as versioned
		if err := ExportVersionedTrace(trace, "json", outputPath, ExportOptions{}, DefaultCompatibilityOptions()); err != nil {
			t.Fatalf("export failed: %v", err)
		}

		// Load back
		loaded, err := LoadVersionedTrace(outputPath, DefaultCompatibilityOptions())
		if err != nil {
			t.Fatalf("LoadVersionedTrace failed: %v", err)
		}

		if loaded.TransactionHash != trace.TransactionHash {
			t.Errorf("transaction hash mismatch")
		}
	})

	t.Run("load legacy trace (no version)", func(t *testing.T) {
		// Legacy traces without version envelope
		legacy := &ExecutionTrace{
			TransactionHash: "legacy-test",
			StartTime:       time.Now(),
			EndTime:         time.Now().Add(time.Minute),
			States:          []ExecutionState{{Step: 0, Operation: "test", Timestamp: time.Now()}},
		}

		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "legacy.json")

		data, _ := json.MarshalIndent(legacy, "", "  ")
		os.WriteFile(path, data, 0o644)

		// Should load successfully as legacy format
		loaded, err := LoadVersionedTrace(path, DefaultCompatibilityOptions())
		if err != nil {
			t.Fatalf("LoadVersionedTrace failed on legacy: %v", err)
		}

		if loaded.TransactionHash != legacy.TransactionHash {
			t.Errorf("transaction hash mismatch in legacy load")
		}
	})

	t.Run("incompatible major version", func(t *testing.T) {
		versioned := VersionedTrace{
			Version: TraceFormatVersion{Major: 99, Minor: 0, Patch: 0},
			Trace: &ExecutionTrace{
				TransactionHash: "future-test",
				StartTime:       time.Now(),
				States:          []ExecutionState{},
			},
		}

		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "future.json")

		data, _ := json.MarshalIndent(versioned, "", "  ")
		os.WriteFile(path, data, 0o644)

		_, err := LoadVersionedTrace(path, DefaultCompatibilityOptions())
		if err == nil {
			t.Fatal("should fail on incompatible major version")
		}
		if !strings.Contains(err.Error(), "incompatible major version") {
			t.Errorf("expected incompatible major version error, got: %v", err)
		}
	})

	t.Run("newer minor version without AllowNewerMinor", func(t *testing.T) {
		versioned := VersionedTrace{
			Version: TraceFormatVersion{Major: 1, Minor: CurrentFormatVersion.Minor + 1, Patch: 0},
			Trace: &ExecutionTrace{
				TransactionHash: "newer-minor",
				StartTime:       time.Now(),
				States:          []ExecutionState{},
			},
		}

		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "newer-minor.json")

		data, _ := json.MarshalIndent(versioned, "", "  ")
		os.WriteFile(path, data, 0o644)

		opts := DefaultCompatibilityOptions()
		opts.AllowNewerMinor = false

		_, err := LoadVersionedTrace(path, opts)
		if err == nil {
			t.Fatal("should fail on newer minor without AllowNewerMinor")
		}
		if !strings.Contains(err.Error(), "newer minor version") {
			t.Errorf("expected newer minor version error, got: %v", err)
		}
	})

	t.Run("strict version check mismatch", func(t *testing.T) {
		versioned := VersionedTrace{
			Version: TraceFormatVersion{Major: 1, Minor: 0, Patch: 1}, // Patch difference
			Trace: &ExecutionTrace{
				TransactionHash: "strict-test",
				StartTime:       time.Now(),
				States:          []ExecutionState{},
			},
		}

		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "strict-test.json")

		data, _ := json.MarshalIndent(versioned, "", "  ")
		os.WriteFile(path, data, 0o644)

		opts := DefaultCompatibilityOptions()
		opts.StrictVersionCheck = true

		_, err := LoadVersionedTrace(path, opts)
		if err == nil {
			t.Fatal("should fail with strict version check and patch mismatch")
		}
		if !strings.Contains(err.Error(), "version mismatch") {
			t.Errorf("expected version mismatch error, got: %v", err)
		}
	})
}

func TestDetectFormat(t *testing.T) {
	trace := NewExecutionTrace("detect-test", 10)
	trace.AddState(ExecutionState{Operation: "test", Timestamp: time.Now()})

	tmpDir := t.TempDir()

	tests := []struct {
		name       string
		format     string
		outputPath string
	}{
		{"json", "json", filepath.Join(tmpDir, "trace.json")},
		{"html", "html", filepath.Join(tmpDir, "trace.html")},
		{"markdown", "markdown", filepath.Join(tmpDir, "trace.md")},
		{"text", "text", filepath.Join(tmpDir, "trace.txt")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Export in format
			if err := ExportExecutionTrace(trace, tt.format, tt.outputPath); err != nil {
				t.Fatalf("export failed: %v", err)
			}

			// Detect format
			detected, err := DetectFormat(tt.outputPath)
			if err != nil {
				t.Fatalf("DetectFormat failed: %v", err)
			}

			expectedFormat := tt.format
			if tt.format == "markdown" {
				expectedFormat = "markdown"
			}

			if detected != expectedFormat && !(tt.format == "markdown" && detected == "markdown") {
				t.Errorf("detected format %q, want %q", detected, expectedFormat)
			}
		})
	}
}

func TestConvertFormat(t *testing.T) {
	trace := NewExecutionTrace("convert-test", 10)
	trace.AddState(ExecutionState{
		Operation:  "test_op",
		ContractID: "test-contract",
		Timestamp:  time.Now(),
	})

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "source.json")

	// Export as JSON first
	if err := ExportExecutionTrace(trace, "json", jsonPath); err != nil {
		t.Fatalf("initial export failed: %v", err)
	}

	t.Run("json to html", func(t *testing.T) {
		htmlPath := filepath.Join(tmpDir, "converted.html")
		if err := ConvertFormat(jsonPath, htmlPath, "html"); err != nil {
			t.Fatalf("ConvertFormat failed: %v", err)
		}

		data, err := os.ReadFile(htmlPath)
		if err != nil {
			t.Fatalf("failed to read converted file: %v", err)
		}

		if !strings.Contains(string(data), "Glassbox Trace Export") {
			t.Error("converted HTML missing expected header")
		}
	})

	t.Run("html to json fails", func(t *testing.T) {
		htmlPath := filepath.Join(tmpDir, "trace.html")
		if err := ExportExecutionTrace(trace, "html", htmlPath); err != nil {
			t.Fatalf("HTML export failed: %v", err)
		}

		err := ConvertFormat(htmlPath, filepath.Join(tmpDir, "from-html.json"), "json")
		if err == nil {
			t.Fatal("converting from HTML should fail")
		}
		if !strings.Contains(err.Error(), "only convert from JSON") {
			t.Errorf("expected 'only convert from JSON' error, got: %v", err)
		}
	})
}

func TestValidateFormatCompatibility(t *testing.T) {
	trace := NewExecutionTrace("compat-test", 10)
	trace.AddState(ExecutionState{
		Operation: "test",
		Timestamp: time.Now(),
	})

	compatOpts := DefaultCompatibilityOptions()

	t.Run("small trace no warnings", func(t *testing.T) {
		warnings := ValidateFormatCompatibility(trace, "html", compatOpts)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings for small trace, got: %v", warnings)
		}
	})

	t.Run("large trace html warning", func(t *testing.T) {
		bigTrace := NewExecutionTrace("big-trace", 10)
		for i := 0; i < 15000; i++ {
			bigTrace.States = append(bigTrace.States, ExecutionState{
				Step:      i,
				Operation: "test",
				Timestamp: time.Now(),
			})
		}

		warnings := ValidateFormatCompatibility(bigTrace, "html", compatOpts)
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "slow HTML rendering") {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected slow rendering warning for large trace, got none")
		}
	})

	t.Run("downgrade warning", func(t *testing.T) {
		opts := compatOpts
		opts.AllowDowngrade = true
		warnings := ValidateFormatCompatibility(trace, "json", opts)
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "Downgrading") {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected downgrade warning")
		}
	})
}

func TestExportWithCompatibility(t *testing.T) {
	trace := NewExecutionTrace("compat-export-test", 10)
	trace.AddState(ExecutionState{
		Operation: "test",
		Timestamp: time.Now(),
	})

	tmpDir := t.TempDir()
	opts := ExportOptions{}
	compatOpts := DefaultCompatibilityOptions()

	t.Run("json export creates versioned file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "compat-json.json")
		if err := ExportWithCompatibility(trace, "json", path, opts, compatOpts); err != nil {
			t.Fatalf("ExportWithCompatibility failed: %v", err)
		}

		data, _ := os.ReadFile(path)
		var versioned VersionedTrace
		if err := json.Unmarshal(data, &versioned); err != nil {
			t.Fatalf("failed to parse as versioned: %v", err)
		}

		if versioned.Version.Major != CurrentFormatVersion.Major {
			t.Error("JSON export should be versioned")
		}
	})

	t.Run("html export works", func(t *testing.T) {
		path := filepath.Join(tmpDir, "compat-html.html")
		if err := ExportWithCompatibility(trace, "html", path, opts, compatOpts); err != nil {
			t.Fatalf("ExportWithCompatibility failed for html: %v", err)
		}

		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), "Glassbox Trace Export") {
			t.Error("HTML export missing expected content")
		}
	})
}

func TestMigrateTrace(t *testing.T) {
	trace := &ExecutionTrace{
		TransactionHash: "migrate-test",
		StartTime:       time.Now(),
		EndTime:         time.Now().Add(time.Minute),
		States: []ExecutionState{
			{Step: 0, Operation: "test", Timestamp: time.Now()},
		},
	}

	fromVersion := TraceFormatVersion{Major: 1, Minor: 0, Patch: 0}
	toVersion := TraceFormatVersion{Major: 1, Minor: 0, Patch: 0}

	migrated, err := migrateTrace(trace, fromVersion, toVersion)
	if err != nil {
		t.Fatalf("migrateTrace failed: %v", err)
	}

	if migrated == nil {
		t.Fatal("migrated trace is nil")
	}

	if migrated.TransactionHash != trace.TransactionHash {
		t.Error("migration should preserve transaction hash")
	}

	// Verify it's a copy, not the same pointer
	if migrated == trace {
		t.Error("migration should return new trace, not original pointer")
	}

	// Test nil migration
	_, err = migrateTrace(nil, fromVersion, toVersion)
	if err == nil {
		t.Fatal("migrateTrace should fail on nil trace")
	}
}
