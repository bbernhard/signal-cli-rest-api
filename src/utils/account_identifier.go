package utils

import (
	"errors"
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
	e164AccountPattern = regexp.MustCompile(`^\+?[1-9]\d{1,14}$`)
	aciAccountPattern  = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// ParseAccountIdentifier accepts an E.164 phone number or an ACI UUID.
// ACI values are returned in lowercase, which is the form signal-cli stores.
func ParseAccountIdentifier(s string) (IdentifierType, string, error) {
	s = strings.TrimSpace(s)
	if e164AccountPattern.MatchString(s) {
		return IdentifierE164, s, nil
	}
	if aciAccountPattern.MatchString(s) {
		return IdentifierACI, strings.ToLower(s), nil
	}
	return 0, "", errors.New("Couldn't process request - invalid account identifier (expected an E.164 phone number or an ACI UUID)")
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
