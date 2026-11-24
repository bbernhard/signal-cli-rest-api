package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/bbernhard/signal-cli-rest-api/utils"
)

// startFakeSignalCli accepts connections like the JSON-RPC socket of signal-cli
// and answers subscribeReceive. It reports the index of the connection on which
// each subscription was made.
func startFakeSignalCli(t *testing.T) (string, <-chan net.Conn, <-chan int) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	conns := make(chan net.Conn, 4)
	subscriptions := make(chan int, 4)
	go func() {
		for i := 0; ; i++ {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conns <- conn
			go func(conn net.Conn, index int) {
				reader := bufio.NewReader(conn)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					var request struct {
						Method string `json:"method"`
						Id     string `json:"id"`
					}
					if json.Unmarshal([]byte(line), &request) != nil {
						continue
					}
					if request.Method == "subscribeReceive" {
						fmt.Fprintf(conn, "{\"jsonrpc\":\"2.0\",\"id\":\"%s\",\"result\":%d}\n", request.Id, index)
						subscriptions <- index
					}
				}
			}(conn, i)
		}
	}()
	return listener.Addr().String(), conns, subscriptions
}

func expectSubscription(t *testing.T, subscriptions <-chan int, connIndex int) {
	t.Helper()
	select {
	case index := <-subscriptions:
		if index != connIndex {
			t.Fatalf("subscribed on connection %d, expected %d", index, connIndex)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("no subscribeReceive on connection %d", connIndex)
	}
}

func TestManualReceiveModeResubscribesAfterReconnect(t *testing.T) {
	const account = "+4915112345678"
	address, conns, subscriptions := startFakeSignalCli(t)

	client := NewJsonRpc2Client(utils.NewSignalCliApiConfig(), account, Manual)
	if err := client.Dial(address, 1); err != nil {
		t.Fatal(err)
	}
	go client.ReceiveData(account, "")

	if _, _, err := client.GetReceiveChannel(account); err != nil {
		t.Fatal(err)
	}
	expectSubscription(t, subscriptions, 0)

	// signal-cli restarts: its subscriptions end with the connection
	(<-conns).Close()
	<-conns

	expectSubscription(t, subscriptions, 1)
}
