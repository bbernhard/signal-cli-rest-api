package client

import (
	"bufio"
	"encoding/json"
	"errors"
	uuid "github.com/gofrs/uuid"
	log "github.com/sirupsen/logrus"
	"net"
)

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type JsonRpc2MessageResponse struct {
	Id     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Err    Error           `json:"error"`
}

type JsonRpc2ReceivedMessage struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Err    Error           `json:"error"`
}

type JsonRpc2Client struct {
	conn                     net.Conn
	receivedMessageResponses chan JsonRpc2MessageResponse
	receivedMessages         chan JsonRpc2ReceivedMessage
}

func NewJsonRpc2Client() *JsonRpc2Client {
	return &JsonRpc2Client{}
}

func (r *JsonRpc2Client) Dial(address string) error {
	var err error
	r.conn, err = net.Dial("tcp", address)
	if err != nil {
		return err
	}

	r.receivedMessageResponses = make(chan JsonRpc2MessageResponse)
	r.receivedMessages = make(chan JsonRpc2ReceivedMessage)

	return nil
}

func (r *JsonRpc2Client) getRaw(command string, args interface{}) (string, error) {
	type Request struct {
		JsonRpc string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		Id      string      `json:"id"`
		Params  interface{} `json:"params,omitempty"`
	}

	u, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	fullCommand := Request{JsonRpc: "2.0", Method: command, Id: u.String()}
	if args != nil {
		fullCommand.Params = args
	}

	fullCommandBytes, err := json.Marshal(fullCommand)
	if err != nil {
		return "", err
	}

	log.Debug("full command: ", string(fullCommandBytes))

	_, err = r.conn.Write([]byte(string(fullCommandBytes) + "\n"))
	if err != nil {
		return "", err
	}

	var resp JsonRpc2MessageResponse
	for {
		resp = <-r.receivedMessageResponses
		if resp.Id == u.String() {
			break
		}
	}

	if resp.Err.Code != 0 {
		return "", errors.New(resp.Err.Message)
	}
	return string(resp.Result), nil
}

func (r *JsonRpc2Client) ReceiveData(number string) {
	connbuf := bufio.NewReader(r.conn)
	for {
		str, err := connbuf.ReadString('\n')
		if err != nil {
			log.Error("Couldn't read data for number ", number, ": ", err.Error(), ". Is the number properly registered?")
			continue
		}
		//log.Info("Received data = ", str)

		var resp1 JsonRpc2ReceivedMessage
		json.Unmarshal([]byte(str), &resp1)
		if resp1.Method == "receive" {
			select {
			case r.receivedMessages <- resp1:
				log.Debug("Message sent to golang channel")
			default:
				log.Debug("Couldn't send message to golang channel, as there's no receiver")
			}
			continue
		}

		var resp2 JsonRpc2MessageResponse
		err = json.Unmarshal([]byte(str), &resp2)
		if err == nil {
			if resp2.Id != "" {
				r.receivedMessageResponses <- resp2
			}
		} else {
			log.Error("Received unparsable message: ", str)
		}
	}
}

//blocks until message a message is received
func (r *JsonRpc2Client) ReceiveMessage() JsonRpc2ReceivedMessage {
	resp := <-r.receivedMessages
	return resp
}
