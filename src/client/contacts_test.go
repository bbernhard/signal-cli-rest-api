package client

import (
	"encoding/json"
	"testing"
)

// TestSignalCliProfileBadgesToProfileBadges verifies that the badges of signal-cli's
// `listContacts` output are carried through to the REST response, including badges
// without an expiration.
func TestSignalCliProfileBadgesToProfileBadges(t *testing.T) {
	const sampleBadgesJSON = `[
	  {"id": "BOOST", "category": "donor", "expirationTimestamp": 1767225600000, "visible": true},
	  {"id": "R_LOW", "category": "donor", "expirationTimestamp": null, "visible": false}
	]`

	var signalCliBadges []SignalCliProfileBadge
	if err := json.Unmarshal([]byte(sampleBadgesJSON), &signalCliBadges); err != nil {
		t.Fatalf("failed to unmarshal sample badges: %v", err)
	}

	badges := signalCliProfileBadgesToProfileBadges(signalCliBadges)
	if len(badges) != 2 {
		t.Fatalf("expected 2 badges, got %d", len(badges))
	}

	first := badges[0]
	if first.Id != "BOOST" || first.Category != "donor" || !first.Visible {
		t.Errorf("unexpected badge: %+v", first)
	}
	if first.ExpirationTimestamp == nil || *first.ExpirationTimestamp != 1767225600000 {
		t.Errorf("unexpected expiration timestamp: %v", first.ExpirationTimestamp)
	}

	second := badges[1]
	if second.Id != "R_LOW" || second.Visible {
		t.Errorf("unexpected badge: %+v", second)
	}
	if second.ExpirationTimestamp != nil {
		t.Errorf("expected no expiration timestamp, got %d", *second.ExpirationTimestamp)
	}
}

// TestSignalCliProfileBadgesToProfileBadgesWithoutBadges verifies that badges are
// serialized as an empty list, rather than null, when signal-cli doesn't report any.
func TestSignalCliProfileBadgesToProfileBadgesWithoutBadges(t *testing.T) {
	encoded, err := json.Marshal(signalCliProfileBadgesToProfileBadges(nil))
	if err != nil {
		t.Fatalf("failed to marshal badges: %v", err)
	}
	if string(encoded) != "[]" {
		t.Errorf("expected [], got %s", encoded)
	}
}
