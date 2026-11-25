package policy

import (
	"httpfin/internal/model"
	"testing"
)

func TestEvaluateRequiresIndependentSignals(t *testing.T) {
	base := model.Connection{ClientHelloSig: "hello", UAConsistency: "Mismatch"}
	if got := Evaluate(base); got.Action != "" {
		t.Fatalf("one signal must not produce a decision: %#v", got)
	}
	base.BaselineStatus = "Mismatch"
	base.HTTP2Fingerprint = "http2-settings"
	got := Evaluate(base)
	if got.Action != "observe" || got.Confidence < 0.9 {
		t.Fatalf("unexpected decision: %#v", got)
	}
}

func TestEvaluateDoesNotTrustUserAgentAlone(t *testing.T) {
	got := Evaluate(model.Connection{DetectedProfile: "Chrome-like", UserAgent: "Chrome"})
	if got.Action != "" {
		t.Fatalf("User-Agent alone must not trigger policy: %#v", got)
	}
}

func TestEvaluateTreatsLikeSuffixAsSameProfile(t *testing.T) {
	got := Evaluate(model.Connection{
		ClientHelloSig:   "hello",
		UAConsistency:    "Mismatch",
		BaselineStatus:   "unknown",
		ReferenceProfile: "Chrome",
		DetectedProfile:  "Chrome-like",
	})
	if got.Action != "" {
		t.Fatalf("equivalent profile names must not create a mismatch decision: %#v", got)
	}
}
