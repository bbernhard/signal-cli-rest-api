package client

import (
	"strings"
	"testing"
)

func TestClassifySignalCliOutputKeepsStderrOutOfResponse(t *testing.T) {
	stdout := "{\"account\":\"+380000000001\"}\n"
	stderr := "INFO Manager - Routine status\nWARN IncomingMessageHandler - Invalid content! reason\njava.lang.Throwable\n\tat example"

	output, infos, warnings := classifySignalCliOutput(stdout, stderr)

	if output != strings.TrimSpace(stdout) {
		t.Fatalf("got output %q, wanted %q", output, strings.TrimSpace(stdout))
	}
	if !strings.Contains(infos, "INFO Manager - Routine status") {
		t.Fatalf("INFO stderr was not preserved at INFO severity: %q", infos)
	}
	if strings.Contains(warnings, "INFO Manager - Routine status") {
		t.Fatalf("INFO stderr was promoted to warning severity: %q", warnings)
	}
	if !strings.Contains(warnings, "Invalid content! reason") || !strings.Contains(warnings, "java.lang.Throwable") {
		t.Fatalf("warnings did not preserve stderr: %q", warnings)
	}
}
