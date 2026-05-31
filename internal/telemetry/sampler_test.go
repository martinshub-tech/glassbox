// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"
)

func TestNewSampler_ClampsRate(t *testing.T) {
	if s := NewSampler(-0.5); s.Rate() != 0 {
		t.Fatalf("expected 0, got %v", s.Rate())
	}
	if s := NewSampler(1.5); s.Rate() != 1.0 {
		t.Fatalf("expected 1.0, got %v", s.Rate())
	}
	if s := NewSampler(0.5); s.Rate() != 0.5 {
		t.Fatalf("expected 0.5, got %v", s.Rate())
	}
}

func TestSampler_RateZero_NeverEmits(t *testing.T) {
	s := NewSampler(0)
	for i := 0; i < 1000; i++ {
		if s.ShouldEmit() {
			t.Fatal("rate=0 sampler must never emit")
		}
	}
}

func TestSampler_RateOne_AlwaysEmits(t *testing.T) {
	s := NewSampler(1.0)
	for i := 0; i < 1000; i++ {
		if !s.ShouldEmit() {
			t.Fatal("rate=1.0 sampler must always emit")
		}
	}
}

func TestSampler_PartialRate_ApproximatelyCorrect(t *testing.T) {
	const n = 100_000
	const rate = 0.1
	const tolerance = 0.02 // ±2%

	s := NewSampler(rate)
	emitted := 0
	for i := 0; i < n; i++ {
		if s.ShouldEmit() {
			emitted++
		}
	}

	got := float64(emitted) / float64(n)
	if got < rate-tolerance || got > rate+tolerance {
		t.Fatalf("expected ~%.2f emission rate, got %.4f", rate, got)
	}
}

func TestSampler_Rate_ReturnsConfiguredValue(t *testing.T) {
	s := NewSampler(0.42)
	if s.Rate() != 0.42 {
		t.Fatalf("expected 0.42, got %v", s.Rate())
	}
}
