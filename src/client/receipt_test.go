package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bbernhard/signal-cli-rest-api/utils"
)

const receiptAccount = "+4915112345678"
const receiptRecipient = "+4915187654321"

func TestSendViewedReceiptInJsonRpcMode(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	params := make(chan map[string]interface{}, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			var request struct {
				Method string                 `json:"method"`
				Id     string                 `json:"id"`
				Params map[string]interface{} `json:"params"`
			}
			if json.Unmarshal([]byte(line), &request) == nil && request.Method == "sendReceipt" {
				params <- request.Params
				fmt.Fprintf(conn, "{\"jsonrpc\":\"2.0\",\"id\":\"%s\",\"result\":{}}\n", request.Id)
			}
		}
	}()

	jsonRpc2Client := NewJsonRpc2Client(utils.NewSignalCliApiConfig(), utils.MULTI_ACCOUNT_NUMBER, OnStart)
	if err := jsonRpc2Client.Dial(listener.Addr().String(), 1); err != nil {
		t.Fatal(err)
	}
	go jsonRpc2Client.ReceiveData(utils.MULTI_ACCOUNT_NUMBER, "")

	signalClient := &SignalClient{
		signalCliMode:   JsonRpc,
		jsonRpc2Clients: map[string]*JsonRpc2Client{utils.MULTI_ACCOUNT_NUMBER: jsonRpc2Client},
	}
	if err := signalClient.SendReceipt(receiptAccount, receiptRecipient, "viewed", 1700000000000); err != nil {
		t.Fatal(err)
	}

	// signal-cli reads the receipt type from "type" and sends a read receipt without it
	if got := (<-params)["type"]; got != "viewed" {
		t.Fatalf("type = %v, expected viewed", got)
	}
}

func TestSendViewedReceiptInNormalMode(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsFile + "\n"
	if err := os.WriteFile(filepath.Join(dir, "signal-cli"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	signalClient := &SignalClient{
		signalCliMode: Normal,
		cliClient:     NewCliClient(Normal, utils.NewSignalCliApiConfig()),
	}
	if err := signalClient.SendReceipt(receiptAccount, receiptRecipient, "viewed", 1700000000000); err != nil {
		t.Fatal(err)
	}

	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--type\nviewed\n") {
		t.Fatalf("signal-cli called without --type viewed: %q", string(args))
	}
}
