package policy

import "httpfin/internal/model"

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
		c.DetectedProfile != c.ReferenceProfile {
		return Decision{Action: "observe", Reason: "ua_reference_profile_mismatch", Confidence: 0.90}
	}
	return Decision{}
}
