package client

import (
	"bufio"
	"net"
	"testing"
	"time"

	"github.com/bbernhard/signal-cli-rest-api/utils"
)

// TestRequestFailsWhenConnectionIsLost makes signal-cli drop the connection while a
// request is waiting for its response. The request must return an error instead of
// waiting forever, which left the REST call (e.g. a send) hanging.
func TestRequestFailsWhenConnectionIsLost(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	go func() {
		// first connection: read one request, then drop the connection
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		bufio.NewReader(conn).ReadString('\n')
		conn.Close()

		// keep the reconnect open so the client keeps running
		conn, err = listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(10 * time.Second)
	}()

	jsonRpc2Client := NewJsonRpc2Client(utils.NewSignalCliApiConfig(), utils.MULTI_ACCOUNT_NUMBER, OnStart)
	if err := jsonRpc2Client.Dial(listener.Addr().String(), 1); err != nil {
		t.Fatal(err)
	}
	go jsonRpc2Client.ReceiveData(utils.MULTI_ACCOUNT_NUMBER, "")

	result := make(chan error, 1)
	go func() {
		_, err := jsonRpc2Client.getRaw("version", nil, nil)
		result <- err
	}()

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("expected an error for a request whose connection was lost")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("request still waiting 5 s after the connection to signal-cli was lost")
	}
}
