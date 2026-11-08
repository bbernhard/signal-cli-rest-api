package client

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/bbernhard/signal-cli-rest-api/utils"
)

// TestVoteInPollSendsIncreasingVoteCounts changes a vote on the same poll and checks
// that the second sendPollVote request carries a higher vote-count than the first.
// Signal clients ignore a vote whose count is not higher than the voter's current one,
// so a constant count means a changed vote never shows up.
func TestVoteInPollSendsIncreasingVoteCounts(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	jsonRpc2Client := NewJsonRpc2Client(utils.NewSignalCliApiConfig(), utils.MULTI_ACCOUNT_NUMBER, OnStart)
	jsonRpc2Client.conn = clientConn
	go jsonRpc2Client.ReceiveData(utils.MULTI_ACCOUNT_NUMBER, "")

	voteCounts := make(chan int64, 2)
	go func() {
		reader := bufio.NewReader(serverConn)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}
			var req struct {
				Id     string `json:"id"`
				Method string `json:"method"`
				Params struct {
					VoteCount int64 `json:"vote-count"`
				} `json:"params"`
			}
			if err := json.Unmarshal(line, &req); err != nil {
				t.Errorf("invalid request %s: %v", line, err)
				return
			}
			if req.Method == "sendPollVote" {
				voteCounts <- req.Params.VoteCount
			}
			// getRaw registers its response channel only after writing the request,
			// so wait for it before answering.
			for registered := false; !registered; {
				jsonRpc2Client.receivedResponsesMutex.Lock()
				_, registered = jsonRpc2Client.receivedResponsesById[req.Id]
				jsonRpc2Client.receivedResponsesMutex.Unlock()
				time.Sleep(time.Millisecond)
			}
			resp, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": req.Id, "result": map[string]interface{}{}})
			if _, err := serverConn.Write(append(resp, '\n')); err != nil {
				return
			}
		}
	}()

	signalClient := &SignalClient{
		signalCliMode:   JsonRpc,
		jsonRpc2Clients: map[string]*JsonRpc2Client{utils.MULTI_ACCOUNT_NUMBER: jsonRpc2Client},
	}

	pollTimestamp := time.Now().Add(-time.Minute).UnixMilli()
	vote := func(answer int32) int64 {
		if err := signalClient.VoteInPoll("+15551230001", "+15551230002", "+15551230002", pollTimestamp, []int32{answer}); err != nil {
			t.Fatalf("VoteInPoll failed: %v", err)
		}
		select {
		case c := <-voteCounts:
			return c
		case <-time.After(5 * time.Second):
			t.Fatal("no sendPollVote request received")
			return 0
		}
	}

	first := vote(2)
	time.Sleep(200 * time.Millisecond)
	second := vote(1)

	if first < 1 {
		t.Fatalf("expected a positive vote count, got %d", first)
	}
	if second <= first {
		t.Fatalf("expected the changed vote to have a higher vote-count than the first vote, got %d then %d", first, second)
	}
}
