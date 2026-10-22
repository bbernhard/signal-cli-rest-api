package utils

import "testing"

func TestParseAccountIdentifierE164(t *testing.T) {
	cases := []string{"+12025550123", "+12", "12025550123"}
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

func TestParseAccountIdentifierInvalid(t *testing.T) {
	cases := []string{
		"",
		"+",
		"+0123",
		"+1",
		"+1234567890123456",
		"+12 345",
		"not-an-account",
		"11111111-1111-4111-8111-11111111111",
		"11111111111141118111111111111111",
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
