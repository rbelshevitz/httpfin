package baseline

import (
	"encoding/json"
	"fmt"
	"httpfin/internal/model"
	"os"
)

type Profile struct {
	Name                  string   `json:"name"`
	Version               string   `json:"version"`
	ClientHelloSignatures []string `json:"client_hello_signatures"`
	HTTP2Fingerprints     []string `json:"http2_fingerprints"`
}

type Set struct {
	Source   string    `json:"source"`
	Profiles []Profile `json:"profiles"`
}

func Load(path string) (*Set, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var set Set
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("parse baseline: %w", err)
	}
	if set.Source == "" {
		set.Source = path
	}
	return &set, nil
}

// Observe enriches a connection without making an access decision. A
// signature known to another declared browser profile is a mismatch signal;
// an unknown signature remains explicitly unknown.
func (s *Set) Observe(c model.Connection) model.Connection {
	if s == nil || c.ClientHelloSig == "" {
		return c
	}
	for _, profile := range s.Profiles {
		if contains(profile.ClientHelloSignatures, c.ClientHelloSig) {
			c.ReferenceProfile = profile.Name
			c.ReferenceVersion = profile.Version
			c.ReferenceSource = s.Source
			if sameProfile(c.DetectedProfile, profile.Name) {
				c.BaselineStatus = "matched"
				c.UAConsistency = "Matched"
			} else {
				c.BaselineStatus = "mismatch"
				c.UAConsistency = "Mismatch"
			}
			return c
		}
	}
	c.BaselineStatus = "unknown"
	c.ReferenceSource = s.Source
	return c
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
	for _, suffix := range []string{"-like", " browser"} {
		for len(value) >= len(suffix) && value[len(value)-len(suffix):] == suffix {
			value = value[:len(value)-len(suffix)]
		}
	}
	return value
}
