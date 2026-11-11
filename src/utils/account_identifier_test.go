package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAccountIdentifierE164(t *testing.T) {
	cases := []string{"+12025550123", "+12"}
	for _, raw := range cases {
		typ, value, err := ParseAccountIdentifier(raw)
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if typ != IdentifierE164 {
			t.Fatalf("%q: type = %v, want E164", raw, typ)
		}
		if value != raw {
			t.Fatalf("%q: value = %q", raw, value)
		}
		flag, selected := SignalCliAccountSelector(typ, value)
		if flag != "-a" || selected != raw {
			t.Fatalf("%q: selector = %s %s", raw, flag, selected)
		}
	}
}

func TestParseAccountIdentifierACI(t *testing.T) {
	raw := "AAAAAAAA-BBBB-4CCC-8DDD-EEEEEEEEEEEE"
	want := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	typ, value, err := ParseAccountIdentifier(raw)
	if err != nil {
		t.Fatal(err)
	}
	if typ != IdentifierACI {
		t.Fatalf("type = %v, want ACI", typ)
	}
	if value != want {
		t.Fatalf("value = %q, want %q", value, want)
	}

	typ, value, err = ParseAccountIdentifier(want)
	if err != nil {
		t.Fatal(err)
	}
	if typ != IdentifierACI || value != want {
		t.Fatalf("lowercase UUID: type=%v value=%q", typ, value)
	}

	flag, selected := SignalCliAccountSelector(typ, value)
	if flag != "-a" || selected != want {
		t.Fatalf("selector = %s %s, want -a %s", flag, selected, want)
	}
}

// signal-cli also accepts the Account Key form: 32 hex characters, optionally grouped by whitespace.
func TestParseAccountIdentifierACIWithoutDashes(t *testing.T) {
	want := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	for _, raw := range []string{"AAAAAAAABBBB4CCC8DDDEEEEEEEEEEEE", "aaaa aaaa bbbb 4ccc 8ddd eeee eeee eeee"} {
		typ, value, err := ParseAccountIdentifier(raw)
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if typ != IdentifierACI || value != want {
			t.Fatalf("%q: type=%v value=%q, want %q", raw, typ, value, want)
		}
	}
}

// writeAccountsJson creates a signal-cli config dir with one numbered and one numberless account.
func writeAccountsJson(t *testing.T) string {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0700); err != nil {
		t.Fatal(err)
	}
	accounts := `{"accounts":[
		{"path":"1","environment":"LIVE","number":"+12025550123","uuid":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"},
		{"path":"2","environment":"LIVE","number":null,"uuid":"11111111-1111-4111-8111-111111111111"}
	],"version":2}`
	if err := os.WriteFile(filepath.Join(dir, "data", "accounts.json"), []byte(accounts), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCanonicalAccount(t *testing.T) {
	dir := writeAccountsJson(t)
	cases := map[string]string{
		"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee": "+12025550123",                         // ACI of a numbered account
		"11111111-1111-4111-8111-111111111111": "11111111-1111-4111-8111-111111111111", // numberless account
		"22222222-2222-4222-8222-222222222222": "22222222-2222-4222-8222-222222222222", // unknown account
		"+12025550123":                         "+12025550123",
	}
	for id, want := range cases {
		if got := CanonicalAccount(dir, id); got != want {
			t.Fatalf("%q: got %q, want %q", id, got, want)
		}
	}
	if got := CanonicalAccount(t.TempDir(), "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"); got != "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee" {
		t.Fatalf("missing accounts.json: got %q", got)
	}
}

func TestTrustModeSharedByNumberAndACI(t *testing.T) {
	config := NewSignalCliApiConfig(writeAccountsJson(t))
	config.SetTrustModeForNumber("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", AlwaysTrust)
	if trustMode, err := config.GetTrustModeForNumber("+12025550123"); err != nil || trustMode != AlwaysTrust {
		t.Fatalf("got %v %v, want AlwaysTrust", trustMode, err)
	}
}

func TestParseAccountIdentifierInvalid(t *testing.T) {
	cases := []string{
		"",
		"+",
		"+0123",
		"+1",
		"+1234567890123456",
		"+12 345",
		"12025550123",
		"not-an-account",
		"11111111-1111-4111-8111-11111111111",
		"GGGGGGGG-1111-4111-8111-111111111111",
	}
	for _, raw := range cases {
		if _, _, err := ParseAccountIdentifier(raw); err == nil {
			t.Fatalf("%q: expected error", raw)
		}
	}
}

func TestParseAccountIdentifierTrimsSpace(t *testing.T) {
	typ, value, err := ParseAccountIdentifier("  +12025550123  ")
	if err != nil {
		t.Fatal(err)
	}
	if typ != IdentifierE164 || value != "+12025550123" {
		t.Fatalf("got %v %q", typ, value)
	}
}
