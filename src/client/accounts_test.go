package client

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestParseListedAccountsNumberless(t *testing.T) {
	raw := `[
		{"number": null, "aci": "11111111-1111-4111-8111-111111111111"},
		{"number": "", "aci": "22222222-2222-4222-8222-222222222222"}
	]`

	accounts, err := ParseListedAccounts(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 {
		t.Fatalf("len = %d, want 2", len(accounts))
	}

	if accounts[0].Number != "" || accounts[0].Identifier() != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("null number: %+v identifier %q", accounts[0], accounts[0].Identifier())
	}
	if accounts[1].Number != "" || accounts[1].Identifier() != "22222222-2222-4222-8222-222222222222" {
		t.Fatalf("empty number: %+v", accounts[1])
	}

	encoded, err := json.Marshal(accounts[0])
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["number"] != "" {
		t.Fatalf("encoded number = %#v, want empty string", decoded["number"])
	}
	if decoded["aci"] != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("encoded aci = %#v", decoded["aci"])
	}
}

func TestParseListedAccountsNumbered(t *testing.T) {
	raw := `[{"number":"+12025550123","aci":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"}]`
	accounts, err := ParseListedAccounts(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 {
		t.Fatalf("len = %d", len(accounts))
	}
	if accounts[0].Number != "+12025550123" {
		t.Fatalf("number = %q", accounts[0].Number)
	}
	if accounts[0].Aci == nil || *accounts[0].Aci != "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee" {
		t.Fatalf("aci = %#v", accounts[0].Aci)
	}
	if accounts[0].Identifier() != "+12025550123" {
		t.Fatalf("identifier = %q", accounts[0].Identifier())
	}
}

func TestParseListedAccountsLegacyNumberOnly(t *testing.T) {
	raw := `[{"number":"+12025550123"}]`
	accounts, err := ParseListedAccounts(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].Number != "+12025550123" || accounts[0].Aci != nil {
		t.Fatalf("legacy account = %+v", accounts)
	}
}

func TestEncodeLinkQRFitsNumberlessLinkURI(t *testing.T) {
	// Version 10 at the highest recovery level holds 976 bits. A numberless
	// link URI that includes capabilities=nopni is longer than that.
	content := "sgnl://linkdevice?uuid=11111111-1111-4111-8111-111111111111&pub_key=" + strings.Repeat("ab", 80) + "&capabilities=nopni"
	if _, err := encodeLinkQR(content, 10); err == nil {
		t.Fatal("version 10 accepted a link URI that is larger than its capacity")
	}
	q, err := encodeLinkQR(content, 0)
	if err != nil {
		t.Fatal(err)
	}
	png, err := q.PNG(256)
	if err != nil {
		t.Fatal(err)
	}
	if len(png) == 0 {
		t.Fatal("encoded QR png is empty")
	}
}

func TestDeviceLinkCommandDoesNotSelectAnAccount(t *testing.T) {
	cmd := deviceLinkCommand("/home/.local/share/signal-cli/", "signal-api")
	want := []string{"--config", "/home/.local/share/signal-cli/", "link", "-n", "signal-api"}
	if !reflect.DeepEqual(cmd, want) {
		t.Fatalf("command = %#v, want %#v", cmd, want)
	}
	for _, arg := range cmd {
		if arg == "-a" || arg == "--account" {
			t.Fatalf("link command must not select an account: %#v", cmd)
		}
	}
}
