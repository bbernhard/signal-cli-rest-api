package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
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

// receiveSubscription tracks the state of a manual-mode signal-cli
// subscribeReceive call: the subscription id assigned by signal-cli and
// a refcount of websocket subscribers attached to that account.
type receiveSubscription struct {
	id       int64
	refcount int
}

type JsonRpc2Client struct {
	conn                       net.Conn
	receivedResponsesById      map[string]chan JsonRpc2MessageResponse
	receivedMessagesChannels   map[string]chan JsonRpc2ReceivedMessage
	receiveSubscriptions       map[string]*receiveSubscription // account -> sub state
	channelAccountByUuid       map[string]string               // channelUuid -> account
	signalCliApiConfig         *utils.SignalCliApiConfig
	number                     string
	receivedMessagesMutex      sync.Mutex
	receivedResponsesMutex     sync.Mutex
	receiveSubscriptionsMutex  sync.Mutex
	address                    string
}

func NewJsonRpc2Client(signalCliApiConfig *utils.SignalCliApiConfig, number string) *JsonRpc2Client {
	return &JsonRpc2Client{
		signalCliApiConfig:       signalCliApiConfig,
		number:                   number,
		receivedResponsesById:    make(map[string]chan JsonRpc2MessageResponse),
		receivedMessagesChannels: make(map[string]chan JsonRpc2ReceivedMessage),
		receiveSubscriptions:     make(map[string]*receiveSubscription),
		channelAccountByUuid:     make(map[string]string),
	}
}

func (r *JsonRpc2Client) Dial(address string, maxRetries int) error {
	var err error
	r.address = address
	connected := false
	for i := 0; i < maxRetries; i++ {
		r.conn, err = net.Dial("tcp", address)
		if err != nil {
			log.Info("Waiting for signal-cli to start up in daemon mode...")
			time.Sleep(2 * time.Second)
			continue
		}

		connected = true
		log.Info("Successfully connected to signal-cli in daemon mode")
		break
	}

	if !connected {
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

func postMessageToWebhook(webhookUrl string, data []byte) error {
	r, err := http.NewRequest("POST", webhookUrl, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	r.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(r)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	log.Info(res.StatusCode)
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return errors.New("Unexpected status code returned (" + strconv.Itoa(res.StatusCode) + ")")
	}
	return nil
}

func (r *JsonRpc2Client) ReceiveData(number string, receiveWebhookUrl string) {
	connbuf := bufio.NewReader(r.conn)
	for {
		str, err := connbuf.ReadString('\n')
		if err != nil {
			log.Error("Lost connection to signal-cli...attempting to reconnect (", err.Error(), ")")
			r.conn.Close()
			err = r.Dial(r.address, 15)
			if err != nil {
				log.Fatal("Unable to reconnect to signal-cli: ", err.Error(), "...aborting")
			}
			connbuf = bufio.NewReader(r.conn)
			log.Info("Successfully reconnected to signal-cli")
			continue
		}
		log.Debug("json-rpc received data: ", str)

		var resp1 JsonRpc2ReceivedMessage
		json.Unmarshal([]byte(str), &resp1)
		if resp1.Method == "receive" {
			// In manual receive-mode signal-cli wraps the envelope in
			// {"subscription":N,"result":{...}}; in auto mode it sends
			// the envelope directly. Unwrap so the broadcast format is
			// the same in both modes and downstream consumers (e.g. the
			// websocket handler) don't have to know which mode is in use.
			var manualWrapper struct {
				Subscription int64           `json:"subscription"`
				Result       json.RawMessage `json:"result"`
			}
			if err := json.Unmarshal(resp1.Params, &manualWrapper); err == nil && len(manualWrapper.Result) > 0 {
				resp1.Params = manualWrapper.Result
			}

			r.receivedMessagesMutex.Lock()
			for _, c := range r.receivedMessagesChannels {
				select {
				case c <- resp1:
					log.Debug("Message sent to golang channel")
				default:
					log.Debug("Couldn't send message to golang channel, as there's no receiver")
				}
			}
			r.receivedMessagesMutex.Unlock()

			if receiveWebhookUrl != "" {
				err = postMessageToWebhook(receiveWebhookUrl, []byte(str))
				if err != nil {
					log.Error("Couldn't post data to webhook: ", err)
				}
			}
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
			log.Warn("Received unparsable message: ", str)
		}
	}
}

// subscribeReceive starts receiving messages for an account by calling
// signal-cli's subscribeReceive JSON-RPC method. Only relevant when
// signal-cli was launched with --receive-mode=manual; in auto mode the
// daemon pushes notifications without an explicit subscribe call.
// Returns the subscription id assigned by signal-cli.
func (r *JsonRpc2Client) subscribeReceive(account string) (int64, error) {
	type subscribeReceiveArgs struct{}
	resultStr, err := r.getRaw("subscribeReceive", &account, subscribeReceiveArgs{})
	if err != nil {
		return 0, err
	}
	var subscriptionId int64
	if err := json.Unmarshal([]byte(resultStr), &subscriptionId); err != nil {
		return 0, fmt.Errorf("subscribeReceive: couldn't parse subscription id from %q: %w", resultStr, err)
	}
	return subscriptionId, nil
}

// unsubscribeReceive cancels a manual-mode subscription previously
// returned by subscribeReceive.
func (r *JsonRpc2Client) unsubscribeReceive(account string, subscriptionId int64) error {
	type unsubscribeReceiveArgs struct {
		Subscription int64 `json:"subscription"`
	}
	_, err := r.getRaw("unsubscribeReceive", &account, unsubscribeReceiveArgs{Subscription: subscriptionId})
	return err
}

// acquireReceiveSubscription ensures an active manual-mode subscription
// exists for the given account, refcounting concurrent websocket
// subscribers. The first caller for an account triggers a real
// subscribeReceive RPC; subsequent callers just bump the refcount.
func (r *JsonRpc2Client) acquireReceiveSubscription(account string) error {
	r.receiveSubscriptionsMutex.Lock()
	defer r.receiveSubscriptionsMutex.Unlock()

	if sub, ok := r.receiveSubscriptions[account]; ok {
		sub.refcount++
		return nil
	}
	id, err := r.subscribeReceive(account)
	if err != nil {
		return err
	}
	r.receiveSubscriptions[account] = &receiveSubscription{id: id, refcount: 1}
	log.Infof("Subscribed to receive notifications for account %s (subscription=%d)", account, id)
	return nil
}

// releaseReceiveSubscription decrements the per-account refcount and
// cancels the subscription with signal-cli if it drops to zero.
func (r *JsonRpc2Client) releaseReceiveSubscription(account string) {
	r.receiveSubscriptionsMutex.Lock()
	defer r.receiveSubscriptionsMutex.Unlock()

	sub, ok := r.receiveSubscriptions[account]
	if !ok {
		return
	}
	sub.refcount--
	if sub.refcount > 0 {
		return
	}
	if err := r.unsubscribeReceive(account, sub.id); err != nil {
		log.Warnf("unsubscribeReceive failed for account %s (subscription=%d): %s", account, sub.id, err.Error())
	} else {
		log.Infof("Unsubscribed from receive notifications for account %s (subscription=%d)", account, sub.id)
	}
	delete(r.receiveSubscriptions, account)
}

// GetReceiveChannel returns a channel that will receive messages for the
// given account. If signal-cli is in manual receive-mode, it also acquires
// a subscription so that signal-cli starts forwarding messages for the
// account; in auto mode, account is unused (notifications flow regardless).
//
// account may be empty when the caller does not need a subscription
// (e.g. legacy callers in auto mode); in that case no subscribeReceive
// RPC is issued.
func (r *JsonRpc2Client) GetReceiveChannel(account string) (chan JsonRpc2ReceivedMessage, string, error) {
	c := make(chan JsonRpc2ReceivedMessage, 64)

	channelUuid, err := uuid.NewV4()
	if err != nil {
		return c, "", err
	}

	if account != "" {
		if err := r.acquireReceiveSubscription(account); err != nil {
			return c, "", fmt.Errorf("subscribeReceive failed for account %s: %w", account, err)
		}
	}

	r.receivedMessagesMutex.Lock()
	r.receivedMessagesChannels[channelUuid.String()] = c
	r.channelAccountByUuid[channelUuid.String()] = account
	r.receivedMessagesMutex.Unlock()

	return c, channelUuid.String(), nil
}

func (r *JsonRpc2Client) RemoveReceiveChannel(channelUuid string) {
	r.receivedMessagesMutex.Lock()
	delete(r.receivedMessagesChannels, channelUuid)
	account := r.channelAccountByUuid[channelUuid]
	delete(r.channelAccountByUuid, channelUuid)
	r.receivedMessagesMutex.Unlock()

	if account != "" {
		r.releaseReceiveSubscription(account)
	}
}
