package policy

import (
	"httpfin/internal/model"
	"strings"
)

type Decision struct {
	Action     string
	Reason     string
	Confidence float64
}

// Evaluate deliberately requires two independent observations. A User-Agent
// string alone is client-controlled and must never produce a deny decision.
func Evaluate(c model.Connection) Decision {
	if c.ClientHelloSig == "" {
		return Decision{}
	}
	if c.UAConsistency == "Mismatch" && c.BaselineStatus == "Mismatch" &&
		c.HTTP2Fingerprint != "" {
		return Decision{Action: "observe", Reason: "ua_tls_baseline_mismatch", Confidence: 0.95}
	}
	if c.UAConsistency == "Mismatch" && c.ReferenceProfile != "" &&
		!sameProfile(c.DetectedProfile, c.ReferenceProfile) {
		return Decision{Action: "observe", Reason: "ua_reference_profile_mismatch", Confidence: 0.90}
	}
	return Decision{}
}

func sameProfile(observed, reference string) bool {
	if observed == "" || reference == "" {
		return false
	}
	observed = normalizeProfile(observed)
	reference = normalizeProfile(reference)
	return observed == reference || observed+"-like" == reference || observed == reference+"-like"
}

func normalizeProfile(value string) string {
	value = strings.TrimSpace(value)
	for _, suffix := range []string{"-like", " browser"} {
		for strings.HasSuffix(value, suffix) {
			value = strings.TrimSuffix(value, suffix)
		}
	}
	return value
}
