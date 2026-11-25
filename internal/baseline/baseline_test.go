package baseline

import (
	"httpfin/internal/model"
	"testing"
)

func TestObserveMatchesAndDetectsDeclaredProfileMismatch(t *testing.T) {
	set := &Set{Source: "fixture", Profiles: []Profile{{Name: "Chrome", Version: "stable", ClientHelloSignatures: []string{"hello-chrome"}}}}
	matched := set.Observe(model.Connection{DetectedProfile: "Chrome-like", ClientHelloSig: "hello-chrome"})
	if matched.BaselineStatus != "matched" || matched.UAConsistency != "Matched" {
		t.Fatalf("unexpected match: %#v", matched)
	}
	mismatch := set.Observe(model.Connection{DetectedProfile: "Firefox-like", ClientHelloSig: "hello-chrome"})
	if mismatch.BaselineStatus != "mismatch" || mismatch.UAConsistency != "Mismatch" || mismatch.ReferenceProfile != "Chrome" {
		t.Fatalf("unexpected mismatch: %#v", mismatch)
	}
}

func TestObserveKeepsUnknownExplicit(t *testing.T) {
	set := &Set{Source: "fixture", Profiles: []Profile{{Name: "Chrome", ClientHelloSignatures: []string{"hello-chrome"}}}}
	got := set.Observe(model.Connection{DetectedProfile: "Chrome-like", ClientHelloSig: "other"})
	if got.BaselineStatus != "unknown" || got.ReferenceSource != "fixture" {
		t.Fatalf("unexpected unknown result: %#v", got)
	}
}
