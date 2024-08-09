package client

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/bbernhard/signal-cli-rest-api/utils"
	uuid "github.com/gofrs/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/sjson"
)

type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
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

type RateLimitMessage struct {
	Response RateLimitResponse `json:"response"`
}

type RateLimitResponse struct {
	Results []RateLimitResult `json:"results"`
}

type RateLimitResult struct {
	Token string `json:"token"`
}

type RateLimitErrorType struct {
	ChallengeTokens []string
	Err             error
}

func (r *RateLimitErrorType) Error() string {
	return r.Err.Error()
}

type JsonRpc2Client struct {
	conn                     net.Conn
	receivedResponsesById    map[string]chan JsonRpc2MessageResponse
	receivedMessagesChannels map[string]chan JsonRpc2ReceivedMessage
	lastTimeErrorMessageSent time.Time
	signalCliApiConfig       *utils.SignalCliApiConfig
	number                   string
	receivedMessagesMutex    sync.Mutex
	receivedResponsesMutex   sync.Mutex
}

func NewJsonRpc2Client(signalCliApiConfig *utils.SignalCliApiConfig, number string) *JsonRpc2Client {
	return &JsonRpc2Client{
		signalCliApiConfig:       signalCliApiConfig,
		number:                   number,
		receivedResponsesById:    make(map[string]chan JsonRpc2MessageResponse),
		receivedMessagesChannels: make(map[string]chan JsonRpc2ReceivedMessage),
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

func (r *JsonRpc2Client) getRaw(command string, account *string, args interface{}) (string, error) {
	type Request struct {
		JsonRpc string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		Id      string      `json:"id"`
		Params  interface{} `json:"params,omitempty"`
	}

	trustModeStr := ""
	trustMode, err := r.signalCliApiConfig.GetTrustModeForNumber(r.number)
	if err == nil {
		trustModeStr, err = utils.TrustModeToString(trustMode)
		if err != nil {
			trustModeStr = ""
			log.Error("Invalid trust mode: ", trustModeStr)
		}
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

	if trustModeStr != "" {
		fullCommandBytes, err = sjson.SetBytes(fullCommandBytes, "params.trustNewIdentities", trustModeStr)
		if err != nil {
			return "", err
		}
	}

	if account != nil {
		fullCommandBytes, err = sjson.SetBytes(fullCommandBytes, "params.account", account)
		if err != nil {
			return "", err
		}
	}

	log.Debug("json-rpc command: ", string(fullCommandBytes))

	_, err = r.conn.Write([]byte(string(fullCommandBytes) + "\n"))
	if err != nil {
		return "", err
	}

	responseChan := make(chan JsonRpc2MessageResponse)
	r.receivedResponsesMutex.Lock()
	r.receivedResponsesById[u.String()] = responseChan
	r.receivedResponsesMutex.Unlock()

	var resp JsonRpc2MessageResponse
	resp = <-responseChan

	r.receivedResponsesMutex.Lock()
	delete(r.receivedResponsesById, u.String())
	r.receivedResponsesMutex.Unlock()

	log.Debug("json-rpc command response message: ", string(resp.Result))
	log.Debug("json-rpc response error: ", string(resp.Err.Message))

	if resp.Err.Code != 0 {
		log.Debug("json-rpc command error code: ", resp.Err.Code)
		if resp.Err.Code == -5 {
			var rateLimitMessage RateLimitMessage
			err = json.Unmarshal(resp.Err.Data, &rateLimitMessage)
			if err != nil {
				return "", errors.New(resp.Err.Message + " (Couldn't parse JSON for more details")
			}
			challengeTokens := []string{}
			for _, rateLimitResult := range rateLimitMessage.Response.Results {
				challengeTokens = append(challengeTokens, rateLimitResult.Token)
			}

			return "", &RateLimitErrorType{
				ChallengeTokens: challengeTokens,
				Err:             errors.New(resp.Err.Message),
			}
		}
		return "", errors.New(resp.Err.Message)
	}

	return string(resp.Result), nil
}

func (r *JsonRpc2Client) ReceiveData(number string) {
	connbuf := bufio.NewReader(r.conn)
	for {
		str, err := connbuf.ReadString('\n')
		if err != nil {
			elapsed := time.Since(r.lastTimeErrorMessageSent)
			if (elapsed) > time.Duration(5*time.Minute) { //avoid spamming the log file and only log the message at max every 5 minutes
				log.Error("Couldn't read data for number ", number, ": ", err.Error(), ". Is the number properly registered?")
				r.lastTimeErrorMessageSent = time.Now()
			}
			continue
		}
		log.Debug("json-rpc received data: ", str)

		var resp1 JsonRpc2ReceivedMessage
		json.Unmarshal([]byte(str), &resp1)
		if resp1.Method == "receive" {
			r.receivedMessagesMutex.Lock()
			for _, c := range r.receivedMessagesChannels {
				select {
				case c <- resp1:
					log.Debug("Message sent to golang channel")
				default:
					log.Debug("Couldn't send message to golang channel, as there's no receiver")
				}
				continue
			}
			r.receivedMessagesMutex.Unlock()
		}

		var resp2 JsonRpc2MessageResponse
		err = json.Unmarshal([]byte(str), &resp2)
		if err == nil {
			if resp2.Id != "" {
				if responseChan, ok := r.receivedResponsesById[resp2.Id]; ok {
					responseChan <- resp2
				}
			}
		} else {
			log.Error("Received unparsable message: ", str)
		}
	}
}

func (r *JsonRpc2Client) GetReceiveChannel() (chan JsonRpc2ReceivedMessage, string, error) {
	c := make(chan JsonRpc2ReceivedMessage)

	channelUuid, err := uuid.NewV4()
	if err != nil {
		return c, "", err
	}

	r.receivedMessagesMutex.Lock()
	r.receivedMessagesChannels[channelUuid.String()] = c
	r.receivedMessagesMutex.Unlock()

	return c, channelUuid.String(), nil
}

func (r *JsonRpc2Client) RemoveReceiveChannel(channelUuid string) {
	r.receivedMessagesMutex.Lock()
	delete(r.receivedMessagesChannels, channelUuid)
	r.receivedMessagesMutex.Unlock()
}
