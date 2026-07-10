package client

import (
	"strings"
	"testing"
)

func TestClassifySignalCliOutputKeepsStderrOutOfResponse(t *testing.T) {
	stdout := "{\"account\":\"+380000000001\"}\n"
	stderr := "WARN IncomingMessageHandler - Invalid content! reason\njava.lang.Throwable\n\tat example"

	output, _, warnings := classifySignalCliOutput(stdout, stderr)

	if output != strings.TrimSpace(stdout) {
		t.Fatalf("got output %q, wanted %q", output, strings.TrimSpace(stdout))
	}
	if !strings.Contains(warnings, "Invalid content! reason") || !strings.Contains(warnings, "java.lang.Throwable") {
		t.Fatalf("warnings did not preserve stderr: %q", warnings)
	}
}
