package client

import (
	"reflect"
	"strings"
	"testing"
)

func TestRedactSecretArgs(t *testing.T) {
	args := []string{"-a", "+12025550123", "register", "--recovery-key", "KEY", "--totp", "123456"}
	want := []string{"-a", "+12025550123", "register", "--recovery-key", "***", "--totp", "***"}
	if got := redactSecretArgs(args); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if args[4] != "KEY" {
		t.Fatal("redactSecretArgs modified the arguments that are passed to signal-cli")
	}
	if got := redactSecretArgs([]string{"-a", "+12025550123", "setPin", "1234"}); got[3] != "***" {
		t.Fatalf("setPin pin not redacted: %q", got)
	}
}

func TestRedactSecretParams(t *testing.T) {
	request := []byte(`{"jsonrpc":"2.0","method":"register","params":{"account":"+12025550123","recoveryKey":"KEY","totp":"123456"}}`)
	original := string(request)

	got := string(redactSecretParams(request))
	if strings.Contains(got, "KEY") || strings.Contains(got, "123456") || !strings.Contains(got, `"recoveryKey":"***"`) {
		t.Fatalf("secrets not redacted: %s", got)
	}
	if string(request) != original {
		t.Fatal("redactSecretParams modified the request that is sent to signal-cli")
	}
	if strings.Contains(got, `"pin"`) {
		t.Fatalf("absent param was added: %s", got)
	}
}
