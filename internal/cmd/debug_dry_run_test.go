// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strings"
	"testing"
)

func TestValidateRPCURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid https url",
			url:     "https://soroban-testnet.stellar.org",
			wantErr: false,
		},
		{
			name:    "valid http url",
			url:     "http://localhost:8000",
			wantErr: false,
		},
		{
			name:    "multiple urls comma separated",
			url:     "https://rpc1.stellar.org,https://rpc2.stellar.org",
			wantErr: false,
		},
		{
			name:    "empty url",
			url:     "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "invalid scheme",
			url:     "ftp://example.com",
			wantErr: true,
			errMsg:  "must use http or https",
		},
		{
			name:    "no scheme",
			url:     "example.com",
			wantErr: true,
			errMsg:  "http or https",
		},
		{
			name:    "malformed url",
			url:     "ht!tp://bad url",
			wantErr: true,
			errMsg:  "invalid URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRPCURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRPCURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateRPCURL() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateSimulatorVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid version",
			version: "1.2.3",
			wantErr: false,
		},
		{
			name:    "valid version with v prefix",
			version: "v2.0.0",
			wantErr: false,
		},
		{
			name:    "development version",
			version: "0.0.1",
			wantErr: true,
			errMsg:  "development build",
		},
		{
			name:    "unknown version",
			version: "unknown",
			wantErr: true,
			errMsg:  "too old",
		},
		{
			name:    "empty version",
			version: "",
			wantErr: true,
			errMsg:  "unable to determine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSimulatorVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSimulatorVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateSimulatorVersion() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateProtocolVersion(t *testing.T) {
	tests := []struct {
		name    string
		version uint32
		wantErr bool
	}{
		{
			name:    "minimum supported version",
			version: 20,
			wantErr: false,
		},
		{
			name:    "middle version",
			version: 21,
			wantErr: false,
		},
		{
			name:    "maximum supported version",
			version: 23,
			wantErr: false,
		},
		{
			name:    "too low",
			version: 19,
			wantErr: true,
		},
		{
			name:    "too high",
			version: 24,
			wantErr: true,
		},
		{
			name:    "way too low",
			version: 10,
			wantErr: true,
		},
		{
			name:    "way too high",
			version: 100,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProtocolVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateProtocolVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "outside the supported range") {
				t.Errorf("validateProtocolVersion() error should mention supported range, got: %v", err)
			}
		})
	}
}

// TestDryRunNetworkValidation tests that network validation provides clear error messages
func TestDryRunNetworkValidation(t *testing.T) {
	// This is an integration-style test that would require mocking the RPC client
	// For now, we'll test the individual validation functions above
	// Full integration tests would be added to integration/ directory
}

// TestDryRunCompareNetworkValidation tests compare-network validation
func TestDryRunCompareNetworkValidation(t *testing.T) {
	// Test case: same network for both primary and compare should fail
	// This would be tested in the full runDebugDryRun function
	// Individual validation is covered above
}
