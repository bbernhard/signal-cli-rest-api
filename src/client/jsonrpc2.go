package client

import (
	"encoding/json"
	"errors"
	"net"
	"bufio"
	uuid "github.com/gofrs/uuid"
	log "github.com/sirupsen/logrus"
)

type JsonRpc2Client struct {
	conn net.Conn
}

func NewJsonRpc2Client() *JsonRpc2Client {
	return &JsonRpc2Client{
	}
}

func (r *JsonRpc2Client) Dial(address string) error {
	var err error
	r.conn, err = net.Dial("tcp", address)
    if err != nil {
        return err
    }

	return nil
}

func (r *JsonRpc2Client) getRaw(command string, args interface{}) (string, error) {
	type Request struct {
		JsonRpc  string `json:"jsonrpc"`
		Method string `json:"method"`
		Id string `json:"id"`
		Params interface{} `json:"params,omitempty"`
	}

	type Error struct {
		Code int `json:"code"`
		Message string `json:"message"`
	}

	type Response struct {
		Id string `json:"id"`
		Result json.RawMessage `json:"result"`
		Err Error `json:"error"`
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

	log.Info("request = ", string(fullCommandBytes))

	_, err = r.conn.Write([]byte(string(fullCommandBytes) + "\n"))
    if err != nil {
        return "", err
    }

	connbuf := bufio.NewReader(r.conn)
	for {
		str, err := connbuf.ReadString('\n')
		if err != nil {
			return "", err
		}

		var resp Response
		err = json.Unmarshal([]byte(str), &resp)
		if err == nil {
			if resp.Id == u.String() {
				log.Info("Response1 = ", string(resp.Result))
				if resp.Err.Code != 0 {
					return "", errors.New(resp.Err.Message)
				}
				return string(resp.Result), nil
			}
		} else {
			log.Info("Response = ", str)
		}
	}

	return "", errors.New("no data")
}
