package utils

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// IdentifierType is the kind of account selector signal-cli accepts.
// E.164 phone numbers and ACI UUIDs are disjoint, so one path parameter can carry either.
type IdentifierType int

const (
	IdentifierE164 IdentifierType = iota
	IdentifierACI
)

var (
	e164AccountPattern = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)
	aciAccountPattern  = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// ParseAccountIdentifier accepts an E.164 phone number (signal-cli requires the leading +) or an ACI.
// Like signal-cli, an ACI may be given with or without dashes and whitespace (e.g. the Account Key
// shown by Signal Android). It is returned as a lowercase UUID with dashes, the form signal-cli stores.
func ParseAccountIdentifier(s string) (IdentifierType, string, error) {
	s = strings.TrimSpace(s)
	if e164AccountPattern.MatchString(s) {
		return IdentifierE164, s, nil
	}
	compact := strings.ToLower(strings.ReplaceAll(strings.Join(strings.Fields(s), ""), "-", ""))
	if aciAccountPattern.MatchString(compact) {
		return IdentifierACI, compact[:8] + "-" + compact[8:12] + "-" + compact[12:16] + "-" + compact[16:20] + "-" + compact[20:], nil
	}
	return 0, "", errors.New("Couldn't process request - invalid account identifier (expected an E.164 phone number with leading + or an ACI)")
}

// CanonicalAccount returns the phone number of the local account whose ACI is id, otherwise id unchanged.
// signal-cli reports numbered accounts by phone number (e.g. in receive events), so mapping the ACI
// to that number gives every account a single key in the API's own bookkeeping.
func CanonicalAccount(signalCliConfigDir string, id string) string {
	if typ, _, err := ParseAccountIdentifier(id); err != nil || typ != IdentifierACI {
		return id
	}

	data, err := os.ReadFile(filepath.Join(signalCliConfigDir, "data", "accounts.json"))
	if err != nil {
		return id
	}
	var stored struct {
		Accounts []struct {
			Number string `json:"number"`
			Uuid   string `json:"uuid"`
		} `json:"accounts"`
	}
	if json.Unmarshal(data, &stored) != nil {
		return id
	}

	for _, account := range stored.Accounts {
		if account.Number != "" && strings.EqualFold(account.Uuid, id) {
			return account.Number
		}
	}
	return id
}

// SignalCliAccountSelector returns the global signal-cli account flag and value.
// Numbered and numberless accounts are both selected with -a/--account.
// -n is the device name on `link` and is not an account selector; `link` must not be given -a.
func SignalCliAccountSelector(typ IdentifierType, value string) (string, string) {
	switch typ {
	case IdentifierE164, IdentifierACI:
		return "-a", value
	default:
		return "-a", value
	}
}
