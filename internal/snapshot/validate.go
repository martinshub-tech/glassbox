// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"fmt"
	"strings"
)

// DriftWarning is a non-fatal diagnostic returned when a snapshot's stored
// fingerprint does not match the fingerprint recomputed from its entries.
// The caller decides whether to surface this as a warning or an error.
type DriftWarning struct {
	Stored   string
	Computed string
}

func (d *DriftWarning) Error() string {
	return fmt.Sprintf(
		"snapshot fingerprint mismatch: stored=%s computed=%s "+
			"(ledger state may be corrupted or was modified after save; "+
			"re-run the debug command to regenerate this snapshot)",
		d.Stored, d.Computed,
	)
}

// LoadWithDiagnostics reads a Snapshot from path and returns both the snapshot
// and any drift warning detected during load.  Unlike Load(), drift is not
// silently logged — it is returned as a *DriftWarning so callers can surface
// the information appropriately.
//
// A non-nil *DriftWarning does NOT prevent the snapshot from being returned;
// callers must choose whether to abort or proceed.
func LoadWithDiagnostics(path string) (*Snapshot, *DriftWarning, error) {
	snap, err := Load(path) // Load already recomputes the fingerprint
	if err != nil {
		return nil, nil, err
	}

	// Load back-fills the fingerprint when it was absent; only report drift
	// when the file explicitly stored a fingerprint that differs from the
	// recomputed value.
	recomputed := ComputeFingerprint(snap)
	if snap.Fingerprint != "" && snap.Fingerprint != recomputed {
		warn := &DriftWarning{Stored: snap.Fingerprint, Computed: recomputed}
		// Align the fingerprint so downstream callers see a consistent value.
		snap.Fingerprint = recomputed
		return snap, warn, nil
	}
	return snap, nil, nil
}

// ValidateSnapshotBeforeReplay performs a comprehensive pre-flight check on a
// persisted snapshot before it is used for replay. It is designed to surface
// actionable errors early — before any simulation work begins.
//
// Checks performed (in order):
//  1. Fingerprint integrity of the ledger state.
//  2. Transaction hash identity (when expectedTxHash is non-empty).
//  3. Network identity (when expectedNetwork is non-empty).
//  4. Staleness against the current CLI params (when currentParams is non-nil).
//  5. WASM source hash staleness (when currentSourceHash is non-empty).
//
// Returns a descriptive, actionable error if any check fails.
func ValidateSnapshotBeforeReplay(
	ps *PersistedSnapshot,
	expectedTxHash string,
	expectedNetwork string,
	currentParams map[string]string,
	currentSourceHash string,
) error {
	if ps == nil {
		return fmt.Errorf("snapshot is nil; re-run the debug command to regenerate it")
	}
	if ps.Metadata == nil {
		return fmt.Errorf(
			"snapshot is missing metadata — the file may be truncated or corrupted; " +
				"re-run the debug command to regenerate the snapshot")
	}
	if ps.Snapshot == nil {
		return fmt.Errorf(
			"snapshot contains no ledger state — the file may be truncated or corrupted; " +
				"re-run the debug command to regenerate the snapshot")
	}

	// 1. Fingerprint check.
	computed := ComputeFingerprint(ps.Snapshot)
	if ps.Snapshot.Fingerprint != "" && ps.Snapshot.Fingerprint != computed {
		return fmt.Errorf(
			"snapshot fingerprint mismatch: stored=%s computed=%s\n"+
				"The ledger state appears to have been modified after the snapshot was saved.\n"+
				"Re-run the debug command to regenerate a valid snapshot",
			ps.Snapshot.Fingerprint, computed,
		)
	}

	// 2. Transaction hash identity.
	if expectedTxHash != "" && ps.Metadata.TxHash != expectedTxHash {
		return fmt.Errorf(
			"snapshot tx hash mismatch: snapshot contains tx=%s but replay requested tx=%s\n"+
				"Use the correct snapshot for this transaction or re-run the debug command",
			ps.Metadata.TxHash, expectedTxHash,
		)
	}

	// 3. Network identity.
	if expectedNetwork != "" && ps.Metadata.Network != expectedNetwork {
		return fmt.Errorf(
			"snapshot network mismatch: snapshot was captured on %q but replay is targeting %q\n"+
				"Re-run the debug command with --network %s to capture a matching snapshot",
			ps.Metadata.Network, expectedNetwork, expectedNetwork,
		)
	}

	// 4 & 5. Staleness.
	if ps.IsStale(currentParams, currentSourceHash) {
		var reasons []string
		if currentParams != nil && ps.Metadata.ParamFingerprint != "" {
			current := hashStringMap(currentParams)
			if ps.Metadata.ParamFingerprint != current {
				reasons = append(reasons, "CLI parameters have changed since the snapshot was saved")
			}
		}
		if currentSourceHash != "" && ps.Metadata.SourceHash != "" &&
			ps.Metadata.SourceHash != currentSourceHash {
			reasons = append(reasons, "WASM source has changed since the snapshot was saved")
		}
		hint := strings.Join(reasons, "; ")
		if hint == "" {
			hint = "snapshot parameters no longer match the current configuration"
		}
		return fmt.Errorf(
			"snapshot is stale: %s\n"+
				"Re-run the debug command to regenerate the snapshot, or pass --snapshot with a "+
				"fresh snapshot file",
			hint,
		)
	}

	return nil
}

// SnapshotLoadDiagnostic returns a human-readable summary of a snapshot's
// state suitable for display in verbose mode or debug output. It is purely
// informational and does not modify the snapshot.
func SnapshotLoadDiagnostic(ps *PersistedSnapshot) string {
	if ps == nil {
		return "snapshot: nil"
	}

	var sb strings.Builder
	sb.WriteString("Snapshot diagnostics:\n")

	if ps.Metadata != nil {
		fmt.Fprintf(&sb, "  Schema version : %d (current: %d)\n",
			ps.Metadata.SchemaVersion, PersistSchemaVersion)
		fmt.Fprintf(&sb, "  Glassbox ver   : %s\n", ps.Metadata.GlassboxVersion)
		fmt.Fprintf(&sb, "  Saved at       : %s\n", ps.Metadata.SavedAt.UTC().Format("2006-01-02T15:04:05Z"))
		fmt.Fprintf(&sb, "  Network        : %s\n", ps.Metadata.Network)
		fmt.Fprintf(&sb, "  Tx hash        : %s\n", ps.Metadata.TxHash)
		if ps.Metadata.SourceHash != "" {
			fmt.Fprintf(&sb, "  WASM hash      : %s\n", ps.Metadata.SourceHash)
		}
		if ps.Metadata.ParamFingerprint != "" {
			fmt.Fprintf(&sb, "  Param hash     : %s\n", ps.Metadata.ParamFingerprint)
		}
	}

	if ps.Snapshot != nil {
		fmt.Fprintf(&sb, "  Ledger entries : %d\n", len(ps.Snapshot.LedgerEntries))
		fmt.Fprintf(&sb, "  Fingerprint    : %s\n", ps.Snapshot.Fingerprint)

		// Validate fingerprint inline.
		computed := ComputeFingerprint(ps.Snapshot)
		if ps.Snapshot.Fingerprint != "" && ps.Snapshot.Fingerprint != computed {
			fmt.Fprintf(&sb, "  [WARN] Fingerprint mismatch! stored=%s computed=%s\n",
				ps.Snapshot.Fingerprint, computed)
		} else {
			sb.WriteString("  Fingerprint OK : matches computed value\n")
		}
	} else {
		sb.WriteString("  [ERROR] Snapshot ledger state is nil\n")
	}

	return sb.String()
}
