package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bbernhard/signal-cli-rest-api/utils"
)

// A webhook handler that calls the API, e.g. to react to the received message,
// must get the response while the webhook request is still open.
func TestWebhookHandlerCanCallTheApi(t *testing.T) {
	const account = "+4915112345678"

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		fmt.Fprintf(conn, "{\"jsonrpc\":\"2.0\",\"method\":\"receive\",\"params\":{\"envelope\":{\"timestamp\":1},\"account\":\"%s\"}}\n", account)
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			var request struct {
				Id string `json:"id"`
			}
			if json.Unmarshal([]byte(line), &request) == nil && request.Id != "" {
				fmt.Fprintf(conn, "{\"jsonrpc\":\"2.0\",\"id\":\"%s\",\"result\":{}}\n", request.Id)
			}
		}
	}()

	jsonRpc2Client := NewJsonRpc2Client(utils.NewSignalCliApiConfig(), account, OnStart)
	if err := jsonRpc2Client.Dial(listener.Addr().String(), 1); err != nil {
		t.Fatal(err)
	}

	result := make(chan string, 1)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		done := make(chan error, 1)
		go func() {
			_, err := jsonRpc2Client.getRaw("sendReaction", &[]string{account}[0], struct{}{})
			done <- err
		}()
		select {
		case err := <-done:
			result <- fmt.Sprint("response: ", err)
		case <-time.After(3 * time.Second):
			result <- "no response while the webhook request was open"
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer webhook.Close()

	go jsonRpc2Client.ReceiveData(account, webhook.URL)

	select {
	case got := <-result:
		if got != "response: <nil>" {
			t.Fatal(got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("webhook was not called")
	}
}
