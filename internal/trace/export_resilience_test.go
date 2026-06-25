// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSanitizeTrace(t *testing.T) {
	tests := []struct {
		name         string
		trace        *ExecutionTrace
		expectErrors int
		checkField   func(*ExecutionTrace) bool
	}{
		{
			name:         "nil trace",
			trace:        nil,
			expectErrors: 1,
		},
		{
			name: "missing start time",
			trace: &ExecutionTrace{
				TransactionHash: "test-hash",
				States:          []ExecutionState{{Operation: "test"}},
			},
			expectErrors: 1,
			checkField: func(t *ExecutionTrace) bool {
				return !t.StartTime.IsZero()
			},
		},
		{
			name: "missing transaction hash",
			trace: &ExecutionTrace{
				StartTime: time.Now(),
				EndTime:   time.Now().Add(time.Minute),
				States:    []ExecutionState{{Operation: "test"}},
			},
			expectErrors: 1,
			checkField: func(t *ExecutionTrace) bool {
				return t.TransactionHash != ""
			},
		},
		{
			name: "step index mismatch",
			trace: &ExecutionTrace{
				TransactionHash: "test-hash",
				StartTime:       time.Now(),
				EndTime:         time.Now().Add(time.Minute),
				States: []ExecutionState{
					{Step: 0, Operation: "test1"},
					{Step: 5, Operation: "test2"}, // Wrong index
				},
			},
			expectErrors: 1,
			checkField: func(t *ExecutionTrace) bool {
				return t.States[1].Step == 1
			},
		},
		{
			name: "missing timestamps in states",
			trace: &ExecutionTrace{
				TransactionHash: "test-hash",
				StartTime:       time.Now(),
				EndTime:         time.Now().Add(time.Minute),
				States: []ExecutionState{
					{Step: 0, Operation: "test1"},
					{Step: 1, Operation: "test2"},
				},
			},
			expectErrors: 2,
			checkField: func(t *ExecutionTrace) bool {
				return !t.States[0].Timestamp.IsZero() && !t.States[1].Timestamp.IsZero()
			},
		},
		{
			name: "overly long error message",
			trace: &ExecutionTrace{
				TransactionHash: "test-hash",
				StartTime:       time.Now(),
				EndTime:         time.Now().Add(time.Minute),
				States: []ExecutionState{
					{
						Step:      0,
						Operation: "test",
						Timestamp: time.Now(),
						Error:     strings.Repeat("x", 15000),
					},
				},
			},
			expectErrors: 1,
			checkField: func(t *ExecutionTrace) bool {
				return len(t.States[0].Error) <= 10000+20 // truncated + "... (truncated)"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized, errs := SanitizeTrace(tt.trace)
			
			if len(errs) != tt.expectErrors {
				t.Errorf("expected %d errors, got %d: %v", tt.expectErrors, len(errs), errs)
			}
			
			if tt.checkField != nil && sanitized != nil {
				if !tt.checkField(sanitized) {
					t.Errorf("sanitized trace failed field check")
				}
			}
		})
	}
}

func TestExportWithResilience(t *testing.T) {
	trace := NewExecutionTrace("test-tx-hash", 10)
	trace.AddState(ExecutionState{
		Operation:  "test_op",
		ContractID: "test-contract",
		Function:   "test_func",
	})

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "resilient-trace.json")

	opts := ExportOptions{}
	recoveryOpts := DefaultRecoveryOptions()
	recoveryOpts.MaxRetries = 2

	err := ExportWithResilience(trace, "json", outputPath, opts, recoveryOpts)
	if err != nil {
		t.Fatalf("ExportWithResilience failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("exported file does not exist")
	}

	// Verify metadata file exists
	metaPath := outputPath + ".meta.json"
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Fatal("metadata file does not exist")
	}

	// Verify content
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}

	var loaded ExecutionTrace
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to unmarshal exported trace: %v", err)
	}

	if loaded.TransactionHash != trace.TransactionHash {
		t.Errorf("transaction hash mismatch: got %s, want %s", loaded.TransactionHash, trace.TransactionHash)
	}
}

func TestAtomicWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "atomic-test.txt")
	content := []byte("test content")

	err := atomicWriteFile(path, content)
	if err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}

	// Verify file exists and has correct content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("content mismatch: got %s, want %s", data, content)
	}

	// Verify no temp files left behind
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp") {
			t.Errorf("temp file not cleaned up: %s", entry.Name())
		}
	}
}

func TestBackupFile(t *testing.T) {
	tmpDir := t.TempDir()
	originalPath := filepath.Join(tmpDir, "original.txt")
	originalContent := []byte("original content")

	// Create original file
	if err := os.WriteFile(originalPath, originalContent, 0o644); err != nil {
		t.Fatalf("failed to create original file: %v", err)
	}

	// Backup the file
	if err := backupFile(originalPath); err != nil {
		t.Fatalf("backupFile failed: %v", err)
	}

	// Find backup file
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}

	backupFound := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "original.txt.bak.") {
			backupFound = true
			
			// Verify backup content
			backupPath := filepath.Join(tmpDir, entry.Name())
			data, err := os.ReadFile(backupPath)
			if err != nil {
				t.Fatalf("failed to read backup file: %v", err)
			}
			
			if string(data) != string(originalContent) {
				t.Errorf("backup content mismatch: got %s, want %s", data, originalContent)
			}
			break
		}
	}

	if !backupFound {
		t.Error("backup file not created")
	}
}

func TestVerifyExport(t *testing.T) {
	trace := NewExecutionTrace("verify-test", 10)
	trace.AddState(ExecutionState{Operation: "test"})

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "verify-trace.json")

	// Export with resilience (creates metadata)
	opts := ExportOptions{}
	recoveryOpts := DefaultRecoveryOptions()
	
	if err := ExportWithResilience(trace, "json", outputPath, opts, recoveryOpts); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// Verify export
	if err := VerifyExport(outputPath); err != nil {
		t.Fatalf("VerifyExport failed: %v", err)
	}

	// Corrupt the file and verify detection
	data, _ := os.ReadFile(outputPath)
	corrupted := append(data, []byte("corrupted")...)
	os.WriteFile(outputPath, corrupted, 0o644)

	err := VerifyExport(outputPath)
	if err == nil {
		t.Fatal("VerifyExport should detect corrupted file")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected checksum mismatch error, got: %v", err)
	}
}

func TestRecoverTrace(t *testing.T) {
	// Create a trace with some issues
	trace := &ExecutionTrace{
		TransactionHash: "recover-test",
		StartTime:       time.Now(),
		EndTime:         time.Now().Add(time.Minute),
		States: []ExecutionState{
			{Step: 0, Operation: "test1", Timestamp: time.Now()},
			{Step: 5, Operation: "test2"}, // Wrong step, missing timestamp
		},
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "recover-trace.json")

	// Write trace to file
	data, _ := json.MarshalIndent(trace, "", "  ")
	os.WriteFile(path, data, 0o644)

	// Recover trace
	recovered, errs := RecoverTrace(path)
	if recovered == nil {
		t.Fatalf("RecoverTrace failed: %v", errs)
	}

	// Should have recovery errors (step mismatch, missing timestamp)
	if len(errs) == 0 {
		t.Error("expected recovery errors for problematic trace")
	}

	// Verify sanitization fixed issues
	if recovered.States[1].Step != 1 {
		t.Errorf("step index not fixed: got %d, want 1", recovered.States[1].Step)
	}

	if recovered.States[1].Timestamp.IsZero() {
		t.Error("timestamp not interpolated")
	}
}

func TestComputeChecksum(t *testing.T) {
	data := []byte("test data")
	checksum1 := computeChecksum(data)
	checksum2 := computeChecksum(data)

	if checksum1 != checksum2 {
		t.Error("checksum not deterministic")
	}

	if len(checksum1) != 64 { // SHA-256 produces 64 hex chars
		t.Errorf("unexpected checksum length: got %d, want 64", len(checksum1))
	}

	// Different data should produce different checksum
	differentData := []byte("different data")
	checksum3 := computeChecksum(differentData)
	if checksum1 == checksum3 {
		t.Error("different data produced same checksum")
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		retryable  bool
	}{
		{
			name:      "nil error",
			err:       nil,
			retryable: false,
		},
		{
			name:      "temporarily unavailable",
			err:       fmt.Errorf("resource temporarily unavailable"),
			retryable: true,
		},
		{
			name:      "connection reset",
			err:       fmt.Errorf("connection reset by peer"),
			retryable: true,
		},
		{
			name:      "broken pipe",
			err:       fmt.Errorf("broken pipe"),
			retryable: true,
		},
		{
			name:      "permission denied",
			err:       fmt.Errorf("permission denied"),
			retryable: false,
		},
		{
			name:      "file not found",
			err:       fmt.Errorf("file not found"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			if result != tt.retryable {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, result, tt.retryable)
			}
		})
	}
}

func TestRecoverFromJSON(t *testing.T) {
	// Valid JSON
	trace := &ExecutionTrace{
		TransactionHash: "json-test",
		StartTime:       time.Now(),
		EndTime:         time.Now().Add(time.Minute),
		States: []ExecutionState{
			{Step: 0, Operation: "test", Timestamp: time.Now()},
		},
	}

	data, _ := json.Marshal(trace)
	
	recovered, err := recoverFromJSON(data)
	if err != nil {
		t.Fatalf("recoverFromJSON failed on valid JSON: %v", err)
	}

	if recovered.TransactionHash != trace.TransactionHash {
		t.Errorf("transaction hash mismatch")
	}

	// Invalid JSON
	invalidData := []byte("{invalid json")
	_, err = recoverFromJSON(invalidData)
	if err == nil {
		t.Error("recoverFromJSON should fail on invalid JSON")
	}
}
