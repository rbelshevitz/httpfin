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
