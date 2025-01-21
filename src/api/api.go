package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"io"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	"github.com/bbernhard/signal-cli-rest-api/client"
	ds "github.com/bbernhard/signal-cli-rest-api/datastructs"
	utils "github.com/bbernhard/signal-cli-rest-api/utils"

	"github.com/yuin/gopher-lua"
	"github.com/cjoudrey/gluahttp"
	"layeh.com/gopher-luar"
	luajson "layeh.com/gopher-json"
)

const (
	// Time allowed to write the file to the client.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the client.
	pongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

type UpdateContactRequest struct {
	Recipient           string  `json:"recipient"`
	Name                *string `json:"name"`
	ExpirationInSeconds *int    `json:"expiration_in_seconds"`
}

type GroupPermissions struct {
	AddMembers string `json:"add_members" enums:"only-admins,every-member"`
	EditGroup  string `json:"edit_group" enums:"only-admins,every-member"`
}

type CreateGroupRequest struct {
	Name           string           `json:"name"`
	Members        []string         `json:"members"`
	Description    string           `json:"description"`
	Permissions    GroupPermissions `json:"permissions"`
	GroupLinkState string           `json:"group_link" enums:"disabled,enabled,enabled-with-approval"`
	ExpirationTime *int             `json:"expiration_time"`
}

type UpdateGroupRequest struct {
	Base64Avatar   *string `json:"base64_avatar"`
	Description    *string `json:"description"`
	Name           *string `json:"name"`
	ExpirationTime *int    `json:"expiration_time"`
}

type ChangeGroupMembersRequest struct {
	Members []string `json:"members"`
}

type ChangeGroupAdminsRequest struct {
	Admins []string `json:"admins"`
}

type LoggingConfiguration struct {
	Level string `json:"Level"`
}

type Configuration struct {
	Logging LoggingConfiguration `json:"logging"`
}

type RegisterNumberRequest struct {
	UseVoice bool   `json:"use_voice"`
	Captcha  string `json:"captcha"`
}

type UnregisterNumberRequest struct {
	DeleteAccount   bool `json:"delete_account" example:"false"`
	DeleteLocalData bool `json:"delete_local_data" example:"false"`
}

type VerifyNumberSettings struct {
	Pin string `json:"pin"`
}

type Reaction struct {
	Recipient    string `json:"recipient"`
	Reaction     string `json:"reaction"`
	TargetAuthor string `json:"target_author"`
	Timestamp    int64  `json:"timestamp"`
}

type Receipt struct {
	Recipient   string `json:"recipient"`
	ReceiptType string `json:"receipt_type" enums:"read,viewed"`
	Timestamp   int64  `json:"timestamp"`
}

type SendMessageV1 struct {
	Number           string   `json:"number"`
	Recipients       []string `json:"recipients"`
	Message          string   `json:"message"`
	Base64Attachment string   `json:"base64_attachment" example:"'<BASE64 ENCODED DATA>' OR 'data:<MIME-TYPE>;base64,<BASE64 ENCODED DATA>' OR 'data:<MIME-TYPE>;filename=<FILENAME>;base64,<BASE64 ENCODED DATA>'"`
	IsGroup          bool     `json:"is_group"`
}

type SendMessageV2 struct {
	Number            string              `json:"number"`
	Recipients        []string            `json:"recipients"`
	Recipient         string              `json:"recipient" swaggerignore:"true"` //some REST API consumers (like the Synology NAS) do not support an array as recipients, so we provide this string parameter here as backup. In order to not confuse anyone, the parameter won't be exposed in the Swagger UI (most users are fine with the recipients parameter).
	Message           string              `json:"message"`
	Base64Attachments []string            `json:"base64_attachments" example:"<BASE64 ENCODED DATA>,data:<MIME-TYPE>;base64<comma><BASE64 ENCODED DATA>,data:<MIME-TYPE>;filename=<FILENAME>;base64<comma><BASE64 ENCODED DATA>"`
	Sticker           string              `json:"sticker"`
	Mentions          []ds.MessageMention `json:"mentions"`
	QuoteTimestamp    *int64              `json:"quote_timestamp"`
	QuoteAuthor       *string             `json:"quote_author"`
	QuoteMessage      *string             `json:"quote_message"`
	QuoteMentions     []ds.MessageMention `json:"quote_mentions"`
	TextMode          *string             `json:"text_mode" enums:"normal,styled"`
	EditTimestamp     *int64              `json:"edit_timestamp"`
	NotifySelf        *bool               `json:"notify_self"`
}

type TypingIndicatorRequest struct {
	Recipient string `json:"recipient"`
}

type Error struct {
	Msg string `json:"error"`
}

type SendMessageError struct {
	Msg             string   `json:"error"`
	ChallengeTokens []string `json:"challenge_tokens,omitempty"`
	Account         string   `json:"account"`
}

type CreateGroupResponse struct {
	Id string `json:"id"`
}

type UpdateProfileRequest struct {
	Name         string  `json:"name"`
	Base64Avatar string  `json:"base64_avatar"`
	About        *string `json:"about"`
}

type TrustIdentityRequest struct {
	VerifiedSafetyNumber *string `json:"verified_safety_number"`
	TrustAllKnownKeys    *bool   `json:"trust_all_known_keys" example:"false"`
}

type SendMessageResponse struct {
	Timestamp string `json:"timestamp"`
}

type TrustModeRequest struct {
	TrustMode string `json:"trust_mode"`
}

type TrustModeResponse struct {
	TrustMode string `json:"trust_mode"`
}

var connectionUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type SearchResponse struct {
	Number     string `json:"number"`
	Registered bool   `json:"registered"`
}

type AddDeviceRequest struct {
	Uri string `json:"uri"`
}

type RateLimitChallengeRequest struct {
	ChallengeToken string `json:"challenge_token" example:"<challenge token>"`
	Captcha        string `json:"captcha" example:"signalcaptcha://{captcha value}"`
}

type UpdateAccountSettingsRequest struct {
	DiscoverableByNumber *bool `json:"discoverable_by_number"`
	ShareNumber          *bool `json:"share_number"`
}

type SetUsernameRequest struct {
	Username string `json:"username" example:"test"`
}

type AddStickerPackRequest struct {
	PackId  string `json:"pack_id" example:"9a32eda01a7a28574f2eb48668ae0dc4"`
	PackKey string `json:"pack_key" example:"19546e18eba0ff69dea78eb591465289d39e16f35e58389ae779d4f9455aff3a"`
}

type Api struct {
	signalClient *client.SignalClient
	wsMutex      sync.Mutex
}

func NewApi(signalClient *client.SignalClient) *Api {
	return &Api{
		signalClient: signalClient,
	}
}

// @Summary Lists general information about the API
// @Tags General
// @Description Returns the supported API versions and the internal build nr
// @Produce  json
// @Success 200 {object} client.About
// @Router /v1/about [get]
func (a *Api) About(c *gin.Context) {
	c.JSON(200, a.signalClient.About())
}

// @Summary Register a phone number.
// @Tags Devices
// @Description Register a phone number with the signal network.
// @Accept  json
// @Produce  json
// @Success 201
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param data body RegisterNumberRequest false "Additional Settings"
// @Router /v1/register/{number} [post]
func (a *Api) RegisterNumber(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	var req RegisterNumberRequest

	buf := new(bytes.Buffer)
	buf.ReadFrom(c.Request.Body)
	if buf.String() != "" {
		err := json.Unmarshal(buf.Bytes(), &req)
		if err != nil {
			log.Error("Couldn't register number: ", err.Error())
			c.JSON(400, Error{Msg: "Couldn't process request - invalid request."})
			return
		}
	} else {
		req.UseVoice = false
		req.Captcha = ""
	}

	if number == "" {
		c.JSON(400, gin.H{"error": "Please provide a number"})
		return
	}

	err = a.signalClient.RegisterNumber(number, req.UseVoice, req.Captcha)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Writer.WriteHeader(201)
}

// @Summary Unregister a phone number.
// @Tags Devices
// @Description Disables push support for this device. **WARNING:** If *delete_account* is set to *true*, the account will be deleted from the Signal Server. This cannot be undone without loss.
// @Accept  json
// @Produce  json
// @Success 204
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param data body UnregisterNumberRequest false "Additional Settings"
// @Router /v1/unregister/{number} [post]
func (a *Api) UnregisterNumber(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	deleteAccount := false
	deleteLocalData := false
	buf := new(bytes.Buffer)
	buf.ReadFrom(c.Request.Body)
	if buf.String() != "" {
		var req UnregisterNumberRequest
		err := json.Unmarshal(buf.Bytes(), &req)
		if err != nil {
			log.Error("Couldn't unregister number: ", err.Error())
			c.JSON(400, Error{Msg: "Couldn't process request - invalid request."})
			return
		}
		deleteAccount = req.DeleteAccount
		deleteLocalData = req.DeleteLocalData
	}

	err = a.signalClient.UnregisterNumber(number, deleteAccount, deleteLocalData)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Writer.WriteHeader(204)
}

// @Summary Verify a registered phone number.
// @Tags Devices
// @Description Verify a registered phone number with the signal network.
// @Accept  json
// @Produce  json
// @Success 201 {string} string "OK"
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param data body VerifyNumberSettings false "Additional Settings"
// @Param token path string true "Verification Code"
// @Router /v1/register/{number}/verify/{token} [post]
func (a *Api) VerifyRegisteredNumber(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	token := c.Param("token")

	pin := ""
	var req VerifyNumberSettings
	buf := new(bytes.Buffer)
	buf.ReadFrom(c.Request.Body)
	if buf.String() != "" {
		err := json.Unmarshal(buf.Bytes(), &req)
		if err != nil {
			log.Error("Couldn't verify number: ", err.Error())
			c.JSON(400, Error{Msg: "Couldn't process request - invalid request."})
			return
		}
		pin = req.Pin
	}

	if number == "" {
		c.JSON(400, gin.H{"error": "Please provide a number"})
		return
	}

	if token == "" {
		c.JSON(400, gin.H{"error": "Please provide a verification code"})
		return
	}

	err = a.signalClient.VerifyRegisteredNumber(number, token, pin)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Writer.WriteHeader(201)
}

// @Summary Send a signal message.
// @Tags Messages
// @Description Send a signal message
// @Accept  json
// @Produce  json
// @Success 201 {string} string "OK"
// @Failure 400 {object} Error
// @Param data body SendMessageV1 true "Input Data"
// @Router /v1/send [post]
// @Deprecated
func (a *Api) Send(c *gin.Context) {

	var req SendMessageV1
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	base64Attachments := []string{}
	if req.Base64Attachment != "" {
		base64Attachments = append(base64Attachments, req.Base64Attachment)
	}

	timestamp, err := a.signalClient.SendV1(req.Number, req.Message, req.Recipients, base64Attachments, req.IsGroup)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.JSON(201, SendMessageResponse{Timestamp: strconv.FormatInt(timestamp.Timestamp, 10)})
}

// @Summary Send a signal message.
// @Tags Messages
// @Description Send a signal message. Set the text_mode to 'styled' in case you want to add formatting to your text message. Styling Options: *italic text*, **bold text**, ~strikethrough text~. If you want to escape a formatting character, prefix it with two backslashes ('\\')
// @Accept  json
// @Produce  json
// @Success 201 {object} SendMessageResponse
// @Failure 400 {object} SendMessageError
// @Param data body SendMessageV2 true "Input Data"
// @Router /v2/send [post]
func (a *Api) SendV2(c *gin.Context) {
	var req SendMessageV2
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, gin.H{"error": "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	//some REST API consumers (like the Synology NAS) do not allow to use an array for the recipients.
	//so, in order to also support those platforms, a fallback parameter (recipient) is provided.
	//this parameter is hidden in the swagger ui in order to not confuse users (most of them are fine with the recipients parameter).
	if req.Recipient != "" {
		req.Recipients = append(req.Recipients, req.Recipient)
	}

	if len(req.Recipients) == 0 {
		c.JSON(400, gin.H{"error": "Couldn't process request - please provide at least one recipient"})
		return
	}

	if req.Number == "" {
		c.JSON(400, gin.H{"error": "Couldn't process request - please provide a valid number"})
		return
	}

	if req.Sticker != "" && !strings.Contains(req.Sticker, ":") {
		c.JSON(400, gin.H{"error": "Couldn't process request - please provide valid sticker delimiter"})
		return
	}

	data, err := a.signalClient.SendV2(
		req.Number, req.Message, req.Recipients, req.Base64Attachments, req.Sticker,
		req.Mentions, req.QuoteTimestamp, req.QuoteAuthor, req.QuoteMessage, req.QuoteMentions,
		req.TextMode, req.EditTimestamp, req.NotifySelf)
	if err != nil {
		switch err.(type) {
		case *client.RateLimitErrorType:
			if rateLimitError, ok := err.(*client.RateLimitErrorType); ok {
				extendedError := errors.New(err.Error() + ". Use the attached challenge tokens to lift the rate limit restrictions via the '/v1/accounts/{number}/rate-limit-challenge' endpoint.")
				c.JSON(429, SendMessageError{Msg: extendedError.Error(), ChallengeTokens: rateLimitError.ChallengeTokens, Account: req.Number})
				return
			} else {
				c.JSON(400, Error{Msg: err.Error()})
				return
			}
		default:
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.JSON(201, SendMessageResponse{Timestamp: strconv.FormatInt((*data)[0].Timestamp, 10)})
}

func (a *Api) handleSignalReceive(ws *websocket.Conn, number string, stop chan struct{}) {
	receiveChannel, channelUuid, err := a.signalClient.GetReceiveChannel()
	if err != nil {
		log.Error("Couldn't get receive channel: ", err.Error())
		return
	}

	for {
		select {
		case <-stop:
			a.signalClient.RemoveReceiveChannel(channelUuid)
			ws.Close()
			return
		case msg := <-receiveChannel:
			var data string = string(msg.Params)
			var err error = nil
			if msg.Err.Code != 0 {
				err = errors.New(msg.Err.Message)
			}

			if err == nil {
				if data != "" {
					type Response struct {
						Account string `json:"account"`
					}
					var response Response
					err = json.Unmarshal([]byte(data), &response)
					if err != nil {
						log.Error("Couldn't parse message ", data, ":", err.Error())
						continue
					}

					if response.Account == number {
						a.wsMutex.Lock()
						err = ws.WriteMessage(websocket.TextMessage, []byte(data))
						if err != nil {
							if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
								log.Error("Couldn't write message: " + err.Error())
							}
							a.wsMutex.Unlock()
							return
						}
						a.wsMutex.Unlock()
					}
				}
			} else {
				errorMsg := Error{Msg: err.Error()}
				errorMsgBytes, err := json.Marshal(errorMsg)
				if err != nil {
					log.Error("Couldn't serialize error message: " + err.Error())
					return
				}
				a.wsMutex.Lock()
				err = ws.WriteMessage(websocket.TextMessage, errorMsgBytes)
				if err != nil {
					log.Error("Couldn't write message: " + err.Error())
					a.wsMutex.Unlock()
					return
				}
				a.wsMutex.Unlock()
			}
		}
	}
}

func wsPong(ws *websocket.Conn, stop chan struct{}) {
	defer func() {
		close(stop)
		ws.Close()
	}()

	ws.SetReadLimit(512)
	ws.SetPongHandler(func(string) error { log.Debug("Received pong"); return nil })
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (a *Api) wsPing(ws *websocket.Conn, stop chan struct{}) {
	pingTicker := time.NewTicker(pingPeriod)
	for {
		select {
		case <-stop:
			ws.Close()
			return
		case <-pingTicker.C:
			a.wsMutex.Lock()
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				a.wsMutex.Unlock()
				return
			}
			a.wsMutex.Unlock()
		}
	}
}

func StringToBool(input string) bool {
	if input == "true" {
		return true
	}
	return false
}

// @Summary Receive Signal Messages.
// @Tags Messages
// @Description Receives Signal Messages from the Signal Network. If you are running the docker container in normal/native mode, this is a GET endpoint. In json-rpc mode this is a websocket endpoint.
// @Accept  json
// @Produce  json
// @Success 200 {object} []string
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param timeout query string false "Receive timeout in seconds (default: 1)"
// @Param ignore_attachments query string false "Specify whether the attachments of the received message should be ignored" (default: false)"
// @Param ignore_stories query string false "Specify whether stories should be ignored when receiving messages" (default: false)"
// @Param max_messages query string false "Specify the maximum number of messages to receive (default: unlimited)". Not available in json-rpc mode.
// @Param send_read_receipts query string false "Specify whether read receipts should be sent when receiving messages" (default: false)"
// @Router /v1/receive/{number} [get]
func (a *Api) Receive(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if a.signalClient.GetSignalCliMode() == client.JsonRpc {
		ws, err := connectionUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
		defer ws.Close()
		var stop = make(chan struct{})
		go a.handleSignalReceive(ws, number, stop)
		go a.wsPing(ws, stop)
		wsPong(ws, stop)
	} else {
		timeout := c.DefaultQuery("timeout", "1")
		timeoutInt, err := strconv.ParseInt(timeout, 10, 32)
		if err != nil {
			c.JSON(400, Error{Msg: "Couldn't process request - timeout needs to be numeric!"})
			return
		}

		maxMessages := c.DefaultQuery("max_messages", "0")
		maxMessagesInt, err := strconv.ParseInt(maxMessages, 10, 32)
		if err != nil {
			c.JSON(400, Error{Msg: "Couldn't process request - max_messages needs to be numeric!"})
			return
		}

		ignoreAttachments := c.DefaultQuery("ignore_attachments", "false")
		if ignoreAttachments != "true" && ignoreAttachments != "false" {
			c.JSON(400, Error{Msg: "Couldn't process request - ignore_attachments parameter needs to be either 'true' or 'false'"})
			return
		}

		ignoreStories := c.DefaultQuery("ignore_stories", "false")
		if ignoreStories != "true" && ignoreStories != "false" {
			c.JSON(400, Error{Msg: "Couldn't process request - ignore_stories parameter needs to be either 'true' or 'false'"})
			return
		}

		sendReadReceipts := c.DefaultQuery("send_read_receipts", "false")
		if sendReadReceipts != "true" && sendReadReceipts != "false" {
			c.JSON(400, Error{Msg: "Couldn't process request - send_read_receipts parameter needs to be either 'true' or 'false'"})
			return
		}

		jsonStr, err := a.signalClient.Receive(number, timeoutInt, StringToBool(ignoreAttachments), StringToBool(ignoreStories), maxMessagesInt, StringToBool(sendReadReceipts))
		if err != nil {
			c.JSON(400, Error{Msg: err.Error()})
			return
		}

		c.String(200, jsonStr)
	}
}

// @Summary Create a new Signal Group.
// @Tags Groups
// @Description Create a new Signal Group with the specified members.
// @Accept  json
// @Produce  json
// @Success 201 {object} CreateGroupResponse
// @Failure 400 {object} Error
// @Param data body CreateGroupRequest true "Input Data"
// @Param number path string true "Registered Phone Number"
// @Router /v1/groups/{number} [post]
func (a *Api) CreateGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	var req CreateGroupRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	editGroupPermission := client.DefaultGroupPermission
	addMembersPermission := client.DefaultGroupPermission
	groupLinkState := client.DefaultGroupLinkState

	if req.Permissions.AddMembers != "" {
		if !utils.StringInSlice(req.Permissions.AddMembers, []string{"every-member", "only-admins"}) {
			c.JSON(400, Error{Msg: "Invalid add members permission provided - only 'every-member' and 'only-admins' allowed!"})
			return
		}
		addMembersPermission = addMembersPermission.FromString(req.Permissions.AddMembers)
	}

	if req.Permissions.EditGroup != "" {
		if !utils.StringInSlice(req.Permissions.EditGroup, []string{"every-member", "only-admins"}) {
			c.JSON(400, Error{Msg: "Invalid edit group permissions provided - only 'every-member' and 'only-admins' allowed!"})
			return
		}
		editGroupPermission = editGroupPermission.FromString(req.Permissions.EditGroup)
	}

	if req.GroupLinkState != "" {
		if !utils.StringInSlice(req.GroupLinkState, []string{"enabled", "enabled-with-approval", "disabled"}) {
			c.JSON(400, Error{Msg: "Invalid group link provided - only 'enabled', 'enabled-with-approval' and 'disabled' allowed!"})
			return
		}
		groupLinkState = groupLinkState.FromString(req.GroupLinkState)
	}

	groupId, err := a.signalClient.CreateGroup(number, req.Name, req.Members, req.Description, editGroupPermission, addMembersPermission, groupLinkState, req.ExpirationTime)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.JSON(201, CreateGroupResponse{Id: groupId})
}

// @Summary Add one or more members to an existing Signal Group.
// @Tags Groups
// @Description Add one or more members to an existing Signal Group.
// @Accept json
// @Produce json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body ChangeGroupMembersRequest true "Members"
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/members [post]
func (a *Api) AddMembersToGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	if groupId == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - group id missing"})
		return
	}

	var req ChangeGroupMembersRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.AddMembersToGroup(number, groupId, req.Members)
	if err != nil {
		switch err.(type) {
		case *client.NotFoundError:
			c.JSON(404, Error{Msg: err.Error()})
			return
		default:
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// @Summary Remove one or more members from an existing Signal Group.
// @Tags Groups
// @Description Remove one or more members from an existing Signal Group.
// @Accept json
// @Produce json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body ChangeGroupMembersRequest true "Members"
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/members [delete]
func (a *Api) RemoveMembersFromGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	if groupId == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - group id missing"})
		return
	}

	var req ChangeGroupMembersRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.RemoveMembersFromGroup(number, groupId, req.Members)
	if err != nil {
		switch err.(type) {
		case *client.NotFoundError:
			c.JSON(404, Error{Msg: err.Error()})
			return
		default:
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// @Summary Add one or more admins to an existing Signal Group.
// @Tags Groups
// @Description Add one or more admins to an existing Signal Group.
// @Accept json
// @Produce json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body ChangeGroupAdminsRequest true "Admins"
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/admins [post]
func (a *Api) AddAdminsToGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	if groupId == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - group id missing"})
		return
	}

	var req ChangeGroupAdminsRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.AddAdminsToGroup(number, groupId, req.Admins)
	if err != nil {
		switch err.(type) {
		case *client.NotFoundError:
			c.JSON(404, Error{Msg: err.Error()})
			return
		default:
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// @Summary Remove one or more admins from an existing Signal Group.
// @Tags Groups
// @Description Remove one or more admins from an existing Signal Group.
// @Accept json
// @Produce json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body ChangeGroupAdminsRequest true "Admins"
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/admins [delete]
func (a *Api) RemoveAdminsFromGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	if groupId == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - group id missing"})
		return
	}

	var req ChangeGroupAdminsRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.RemoveAdminsFromGroup(number, groupId, req.Admins)
	if err != nil {
		switch err.(type) {
		case *client.NotFoundError:
			c.JSON(404, Error{Msg: err.Error()})
			return
		default:
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// @Summary List all Signal Groups.
// @Tags Groups
// @Description List all Signal Groups.
// @Accept  json
// @Produce  json
// @Success 200 {object} []client.GroupEntry
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Router /v1/groups/{number} [get]
func (a *Api) GetGroups(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	groups, err := a.signalClient.GetGroups(number)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.JSON(200, groups)
}

// @Summary List a Signal Group.
// @Tags Groups
// @Description List a specific Signal Group.
// @Accept  json
// @Produce  json
// @Success 200 {object} client.GroupEntry
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid} [get]
func (a *Api) GetGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	groupId := c.Param("groupid")

	groupEntry, err := a.signalClient.GetGroup(number, groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	if groupEntry != nil {
		c.JSON(200, groupEntry)
	} else {
		c.JSON(404, Error{Msg: "No group with that id found"})
	}
}

// @Summary Delete a Signal Group.
// @Tags Groups
// @Description Delete the specified Signal Group.
// @Accept  json
// @Produce  json
// @Success 200 {string} string "OK"
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group Id"
// @Router /v1/groups/{number}/{groupid} [delete]
func (a *Api) DeleteGroup(c *gin.Context) {
	base64EncodedGroupId := c.Param("groupid")
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if base64EncodedGroupId == "" {
		c.JSON(400, Error{Msg: "Please specify a group id"})
		return
	}

	groupId, err := client.ConvertGroupIdToInternalGroupId(base64EncodedGroupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	err = a.signalClient.DeleteGroup(number, groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
}

// @Summary Link device and generate QR code.
// @Tags Devices
// @Description Link device and generate QR code
// @Produce  json
// @Success 200 {string} string	"Image"
// @Param device_name query string true "Device Name"
// @Param qrcode_version query int false "QRCode Version (defaults to 10)"
// @Failure 400 {object} Error
// @Router /v1/qrcodelink [get]
func (a *Api) GetQrCodeLink(c *gin.Context) {
	deviceName := c.Query("device_name")
	qrCodeVersion := c.Query("qrcode_version")

	if deviceName == "" {
		c.JSON(400, Error{Msg: "Please provide a name for the device"})
		return
	}

	qrCodeVersionInt := 10
	if qrCodeVersion != "" {
		var err error
		qrCodeVersionInt, err = strconv.Atoi(qrCodeVersion)
		if err != nil {
			c.JSON(400, Error{Msg: "The qrcode_version parameter needs to be an integer!"})
			return
		}
	}

	png, err := a.signalClient.GetQrCodeLink(deviceName, qrCodeVersionInt)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Data(200, "image/png", png)
}

// @Summary List all accounts
// @Tags Accounts
// @Description Lists all of the accounts linked or registered
// @Produce json
// @Success 200 {object} []string
// @Failure 400 {object} Error
// @Router /v1/accounts [get]
func (a *Api) GetAccounts(c *gin.Context) {
	devices, err := a.signalClient.GetAccounts()
	if err != nil {
		c.JSON(500, Error{Msg: "Couldn't get list of accounts: " + err.Error()})
		return
	}

	c.JSON(200, devices)
}

// @Summary List all attachments.
// @Tags Attachments
// @Description List all downloaded attachments
// @Produce  json
// @Success 200 {object} []string
// @Failure 400 {object} Error
// @Router /v1/attachments [get]
func (a *Api) GetAttachments(c *gin.Context) {
	files, err := a.signalClient.GetAttachments()
	if err != nil {
		c.JSON(500, Error{Msg: "Couldn't get list of attachments: " + err.Error()})
		return
	}

	c.JSON(200, files)
}

// @Summary Remove attachment.
// @Tags Attachments
// @Description Remove the attachment with the given id from filesystem.
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param attachment path string true "Attachment ID"
// @Router /v1/attachments/{attachment} [delete]
func (a *Api) RemoveAttachment(c *gin.Context) {
	attachment := c.Param("attachment")

	err := a.signalClient.RemoveAttachment(attachment)
	if err != nil {
		switch err.(type) {
		case *client.InvalidNameError:
			c.JSON(400, Error{Msg: err.Error()})
			return
		case *client.NotFoundError:
			c.JSON(404, Error{Msg: err.Error()})
			return
		case *client.InternalError:
			c.JSON(500, Error{Msg: err.Error()})
			return
		default:
			c.JSON(500, Error{Msg: err.Error()})
			return
		}
	}

	c.Status(http.StatusNoContent)
}

// @Summary Serve Attachment.
// @Tags Attachments
// @Description Serve the attachment with the given id
// @Produce  json
// @Success 200 {string} OK
// @Failure 400 {object} Error
// @Param attachment path string true "Attachment ID"
// @Router /v1/attachments/{attachment} [get]
func (a *Api) ServeAttachment(c *gin.Context) {
	attachment := c.Param("attachment")

	attachmentBytes, err := a.signalClient.GetAttachment(attachment)
	if err != nil {
		switch err.(type) {
		case *client.InvalidNameError:
			c.JSON(400, Error{Msg: err.Error()})
			return
		case *client.NotFoundError:
			c.JSON(404, Error{Msg: err.Error()})
			return
		case *client.InternalError:
			c.JSON(500, Error{Msg: err.Error()})
			return
		default:
			c.JSON(500, Error{Msg: err.Error()})
			return
		}
	}

	mime, err := mimetype.DetectReader(bytes.NewReader(attachmentBytes))
	if err != nil {
		c.JSON(500, Error{Msg: "Couldn't detect MIME type for attachment"})
		return
	}

	c.Writer.Header().Set("Content-Type", mime.String())
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(attachmentBytes)))
	_, err = c.Writer.Write(attachmentBytes)
	if err != nil {
		c.JSON(500, Error{Msg: "Couldn't serve attachment - please try again later"})
		return
	}
}

// @Summary Update Profile.
// @Tags Profiles
// @Description Set your name and optional an avatar.
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body UpdateProfileRequest true "Profile Data"
// @Param number path string true "Registered Phone Number"
// @Router /v1/profiles/{number} [put]
func (a *Api) UpdateProfile(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req UpdateProfileRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	if req.Name == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - profile name missing"})
		return
	}

	err = a.signalClient.UpdateProfile(number, req.Name, req.Base64Avatar, req.About)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary API Health Check
// @Tags General
// @Description Internally used by the docker container to perform the health check.
// @Produce  json
// @Success 204 {string} OK
// @Router /v1/health [get]
func (a *Api) Health(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// @Summary List Identities
// @Tags Identities
// @Description List all identities for the given number.
// @Produce  json
// @Success 200 {object} []client.IdentityEntry
// @Param number path string true "Registered Phone Number"
// @Router /v1/identities/{number} [get]
func (a *Api) ListIdentities(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	identityEntries, err := a.signalClient.ListIdentities(number)
	if err != nil {
		c.JSON(500, Error{Msg: err.Error()})
		return
	}

	c.JSON(200, identityEntries)
}

// @Summary Trust Identity
// @Tags Identities
// @Description Trust an identity. When 'trust_all_known_keys' is set to' true', all known keys of this user are trusted. **This is only recommended for testing.**
// @Produce  json
// @Success 204 {string} OK
// @Param data body TrustIdentityRequest true "Input Data"
// @Param number path string true "Registered Phone Number"
// @Param numberToTrust path string true "Number To Trust"
// @Router /v1/identities/{number}/trust/{numberToTrust} [put]
func (a *Api) TrustIdentity(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	numberToTrust := c.Param("numbertotrust")
	if numberToTrust == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number to trust missing"})
		return
	}

	var req TrustIdentityRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	if (req.VerifiedSafetyNumber == nil && req.TrustAllKnownKeys == nil) || (req.VerifiedSafetyNumber == nil && req.TrustAllKnownKeys != nil && !*req.TrustAllKnownKeys) {
		c.JSON(400, Error{Msg: "Couldn't process request - please either provide a safety number (preferred & more secure) or set 'trust_all_known_keys' to true"})
		return
	}

	if req.VerifiedSafetyNumber != nil && req.TrustAllKnownKeys != nil && *req.TrustAllKnownKeys {
		c.JSON(400, Error{Msg: "Couldn't process request - please either provide a safety number or set 'trust_all_known_keys' to true. But do not set both parameters at once!"})
		return
	}

	if req.VerifiedSafetyNumber != nil && *req.VerifiedSafetyNumber == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - please provide a valid safety number"})
		return
	}

	err = a.signalClient.TrustIdentity(number, numberToTrust, req.VerifiedSafetyNumber, req.TrustAllKnownKeys)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Set the REST API configuration.
// @Tags General
// @Description Set the REST API configuration.
// @Accept  json
// @Produce  json
// @Success 204 {string} string "OK"
// @Failure 400 {object} Error
// @Param data body Configuration true "Configuration"
// @Router /v1/configuration [post]
func (a *Api) SetConfiguration(c *gin.Context) {
	var req Configuration
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	if req.Logging.Level != "" {
		if req.Logging.Level == "debug" {
			log.SetLevel(log.DebugLevel)
		} else if req.Logging.Level == "info" {
			log.SetLevel(log.InfoLevel)
		} else if req.Logging.Level == "warn" {
			log.SetLevel(log.WarnLevel)
		} else {
			c.JSON(400, Error{Msg: "Couldn't set log level - invalid log level"})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// @Summary List the REST API configuration.
// @Tags General
// @Description List the REST API configuration.
// @Accept  json
// @Produce  json
// @Success 200 {object} Configuration
// @Failure 400 {object} Error
// @Router /v1/configuration [get]
func (a *Api) GetConfiguration(c *gin.Context) {
	logLevel := ""
	if log.GetLevel() == log.DebugLevel {
		logLevel = "debug"
	} else if log.GetLevel() == log.InfoLevel {
		logLevel = "info"
	} else if log.GetLevel() == log.WarnLevel {
		logLevel = "warn"
	}

	configuration := Configuration{Logging: LoggingConfiguration{Level: logLevel}}
	c.JSON(200, configuration)
}

// @Summary Block a Signal Group.
// @Tags Groups
// @Description Block the specified Signal Group.
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/block [post]
func (a *Api) BlockGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	internalGroupId, err := client.ConvertGroupIdToInternalGroupId(groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	err = a.signalClient.BlockGroup(number, internalGroupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Join a Signal Group.
// @Tags Groups
// @Description Join the specified Signal Group.
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/join [post]
func (a *Api) JoinGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	internalGroupId, err := client.ConvertGroupIdToInternalGroupId(groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	err = a.signalClient.JoinGroup(number, internalGroupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Join a Signal Group by invite link.
// @Tags Groups
// @Description Join the specified Signal Group by invite link.
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Query invite_link query string true "Invite Link"
// @Router /v1/groups/{number}/join_by_invite_link [post]
func (a *Api) JoinGroupByInviteLink(c *gin.Context) {
	number := c.Param("number")
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	inviteLink := c.Query("invite_link")
	err := a.signalClient.JoinGroupByInviteLink(number, inviteLink)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Get a Signal Group info by invite link.
// @Tags Groups
// @Description Get the specified Signal Group info by invite link.
// @Accept  json
// @Produce  json
// @Success 200 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Query invite_link query string true "Invite Link"
// @Router /v1/groups/{number}/join_info_by_invite_link [get]
func (a *Api) GetJoinGroupInfoByInviteLink(c *gin.Context) {
	number := c.Param("number")
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	inviteLink := c.Query("invite_link")
	jsonStr, err := a.signalClient.GetJoinGroupInfoByInviteLink(number, inviteLink)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.String(200, jsonStr)
}

// @Summary Quit a Signal Group.
// @Tags Groups
// @Description Quit the specified Signal Group.
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/quit [post]
func (a *Api) QuitGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	internalGroupId, err := client.ConvertGroupIdToInternalGroupId(groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	err = a.signalClient.QuitGroup(number, internalGroupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Update the state of a Signal Group.
// @Tags Groups
// @Description Update the state of a Signal Group.
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Param data body UpdateGroupRequest true "Input Data"
// @Router /v1/groups/{number}/{groupid} [put]
func (a *Api) UpdateGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	internalGroupId, err := client.ConvertGroupIdToInternalGroupId(groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	var req UpdateGroupRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	err = a.signalClient.UpdateGroup(number, internalGroupId, req.Base64Avatar, req.Description, req.Name, req.ExpirationTime)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Send a reaction.
// @Tags Reactions
// @Description React to a message
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body Reaction true "Reaction"
// @Param number path string true "Registered phone number"
// @Router /v1/reactions/{number} [post]
func (a *Api) SendReaction(c *gin.Context) {
	var req Reaction
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if req.Recipient == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - recipient missing"})
		return
	}

	if req.Reaction == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - reaction missing"})
		return
	}

	if req.TargetAuthor == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - target_author missing"})
		return
	}

	if req.Timestamp == 0 {
		c.JSON(400, Error{Msg: "Couldn't process request - timestamp missing"})
		return
	}

	err = a.signalClient.SendReaction(number, req.Recipient, req.Reaction, req.TargetAuthor, req.Timestamp, false)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Remove a reaction.
// @Tags Reactions
// @Description Remove a reaction
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body Reaction true "Reaction"
// @Param number path string true "Registered phone number"
// @Router /v1/reactions/{number} [delete]
func (a *Api) RemoveReaction(c *gin.Context) {
	var req Reaction
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if req.Recipient == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - recipient missing"})
		return
	}

	if req.TargetAuthor == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - target_author missing"})
		return
	}

	if req.Timestamp == 0 {
		c.JSON(400, Error{Msg: "Couldn't process request - timestamp missing"})
		return
	}

	err = a.signalClient.SendReaction(number, req.Recipient, req.Reaction, req.TargetAuthor, req.Timestamp, true)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Send a receipt.
// @Tags Receipts
// @Description Send a read or viewed receipt
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body Receipt true "Receipt"
// @Param number path string true "Registered phone number"
// @Router /v1/receipts/{number} [post]
func (a *Api) SendReceipt(c *gin.Context) {
	var req Receipt
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	if req.Recipient == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - recipient missing"})
		return
	}

	// if req.ReceiptType != "viewed" && req.ReceiptType != "read" {
	if !utils.StringInSlice(req.ReceiptType, []string{"read", "viewed"}) {
		c.JSON(400, Error{Msg: "Couldn't process request - receipt type must be read or viewed"})
		return
	}

	if req.Timestamp == 0 {
		c.JSON(400, Error{Msg: "Couldn't process request - timestamp missing"})
		return
	}

	err = a.signalClient.SendReceipt(number, req.Recipient, req.ReceiptType, req.Timestamp)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Show Typing Indicator.
// @Tags Messages
// @Description Show Typing Indicator.
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param data body TypingIndicatorRequest true "Type"
// @Router /v1/typing-indicator/{number} [put]
func (a *Api) SendStartTyping(c *gin.Context) {
	var req TypingIndicatorRequest
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	err = a.signalClient.SendStartTyping(number, req.Recipient)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Hide Typing Indicator.
// @Tags Messages
// @Description Hide Typing Indicator.
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param data body TypingIndicatorRequest true "Type"
// @Router /v1/typing-indicator/{number} [delete]
func (a *Api) SendStopTyping(c *gin.Context) {
	var req TypingIndicatorRequest
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	err = a.signalClient.SendStopTyping(number, req.Recipient)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Check if one or more phone numbers are registered with the Signal Service.
// @Tags Search
// @Description Check if one or more phone numbers are registered with the Signal Service.
// @Accept  json
// @Produce  json
// @Param number path string false "Registered Phone Number"
// @Param numbers query []string true "Numbers to check" collectionFormat(multi)
// @Success 200 {object} []SearchResponse
// @Failure 400 {object} Error
// @Router /v1/search/{number} [get]
func (a *Api) SearchForNumbers(c *gin.Context) {
	query := c.Request.URL.Query()
	if _, ok := query["numbers"]; !ok {
		c.JSON(400, Error{Msg: "Please provide numbers to query for"})
		return
	}

	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	searchResults, err := a.signalClient.SearchForNumbers(number, query["numbers"])
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	searchResponse := []SearchResponse{}
	for _, val := range searchResults {
		entry := SearchResponse{Number: val.Number, Registered: val.Registered}
		searchResponse = append(searchResponse, entry)
	}

	c.JSON(200, searchResponse)
}

// @Summary Updates the info associated to a number on the contact list. If the contact doesn’t exist yet, it will be added.
// @Tags Contacts
// @Description Updates the info associated to a number on the contact list.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Success 204
// @Param data body UpdateContactRequest true "Contact"
// @Failure 400 {object} Error
// @Router /v1/contacts/{number} [put]
func (a *Api) UpdateContact(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req UpdateContactRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	if req.Recipient == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - recipient missing"})
		return
	}

	err = a.signalClient.UpdateContact(number, req.Recipient, req.Name, req.ExpirationInSeconds)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Links another device to this device.
// @Tags Devices
// @Description Links another device to this device. Only works, if this is the master device.
// @Accept json
// @Produce json
// @Param number path string true "Registered Phone Number"
// @Success 204
// @Param data body AddDeviceRequest true "Request"
// @Failure 400 {object} Error
// @Router /v1/devices/{number} [post]
func (a *Api) AddDevice(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req AddDeviceRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.AddDevice(number, req.Uri)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Set account specific settings.
// @Tags General
// @Description Set account specific settings.
// @Accept json
// @Produce json
// @Param number path string true "Registered Phone Number"
// @Success 204
// @Param data body TrustModeRequest true "Request"
// @Failure 400 {object} Error
// @Router /v1/configuration/{number}/settings [post]
func (a *Api) SetTrustMode(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req TrustModeRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	trustMode, err := utils.StringToTrustMode(req.TrustMode)
	if err != nil {
		c.JSON(400, Error{Msg: "Invalid trust mode"})
		return
	}

	err = a.signalClient.SetTrustMode(number, trustMode)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't set trust mode"})
		log.Error("Couldn't set trust mode: ", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary List account specific settings.
// @Tags General
// @Description List account specific settings.
// @Accept json
// @Produce json
// @Param number path string true "Registered Phone Number"
// @Success 200
// @Param data body TrustModeResponse true "Request"
// @Failure 400 {object} Error
// @Router /v1/configuration/{number}/settings [get]
func (a *Api) GetTrustMode(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	trustMode := TrustModeResponse{}
	trustMode.TrustMode, err = utils.TrustModeToString(a.signalClient.GetTrustMode(number))
	if err != nil {
		c.JSON(400, Error{Msg: "Invalid trust mode"})
		log.Error("Invalid trust mode: ", err.Error())
		return
	}

	c.JSON(200, trustMode)
}

// @Summary Send a synchronization message with the local contacts list to all linked devices.
// @Tags Contacts
// @Description Send a synchronization message with the local contacts list to all linked devices. This command should only be used if this is the primary device.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Success 204
// @Failure 400 {object} Error
// @Router /v1/contacts/{number}/sync [post]
func (a *Api) SendContacts(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	err = a.signalClient.SendContacts(number)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Lift rate limit restrictions by solving a captcha.
// @Tags Accounts
// @Description When running into rate limits, sometimes the limit can be lifted, by solving a CAPTCHA. To get the captcha token, go to https://signalcaptchas.org/challenge/generate.html For the staging environment, use: https://signalcaptchas.org/staging/registration/generate.html. The "challenge_token" is the token from the failed send attempt. The "captcha" is the captcha result, starting with signalcaptcha://
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Param data body RateLimitChallengeRequest true "Request"
// @Success 204
// @Failure 400 {object} Error
// @Router /v1/accounts/{number}/rate-limit-challenge [post]
func (a *Api) SubmitRateLimitChallenge(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req RateLimitChallengeRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.SubmitRateLimitChallenge(number, req.ChallengeToken, req.Captcha)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Update the account settings.
// @Tags Accounts
// @Description Update the account attributes on the signal server.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Param data body UpdateAccountSettingsRequest true "Request"
// @Success 204
// @Failure 400 {object} Error
// @Router /v1/accounts/{number}/settings [put]
func (a *Api) UpdateAccountSettings(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req UpdateAccountSettingsRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.UpdateAccountSettings(number, req.DiscoverableByNumber, req.ShareNumber)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(201)
}

// @Summary Set a username.
// @Tags Accounts
// @Description Allows to set the username that should be used for this account. This can either be just the nickname (e.g. test) or the complete username with discriminator (e.g. test.123). Returns the new username with discriminator and the username link.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Param data body SetUsernameRequest true "Request"
// @Success 201 {object} client.SetUsernameResponse
// @Success 204
// @Failure 400 {object} Error
// @Router /v1/accounts/{number}/username [post]
func (a *Api) SetUsername(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req SetUsernameRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	resp, err := a.signalClient.SetUsername(number, req.Username)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.JSON(201, resp)
}

// @Summary Remove a username.
// @Tags Accounts
// @Description Delete the username associated with this account.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Success 204
// @Failure 400 {object} Error
// @Router /v1/accounts/{number}/username [delete]
func (a *Api) RemoveUsername(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	err = a.signalClient.RemoveUsername(number)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary List Installed Sticker Packs.
// @Tags Sticker Packs
// @Description List Installed Sticker Packs.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Success 204
// @Failure 400 {object} Error
// @Success 200 {object} []client.ListInstalledStickerPacksResponse
// @Router /v1/sticker-packs/{number} [get]
func (a *Api) ListInstalledStickerPacks(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	installedStickerPacks, err := a.signalClient.ListInstalledStickerPacks(number)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.JSON(200, installedStickerPacks)
}

// @Summary Add Sticker Pack.
// @Tags Sticker Packs
// @Description In order to add a sticker pack, browse to https://signalstickers.org/ and select the sticker pack you want to add. Then, press the "Add to Signal" button. If you look at the address bar in your browser you should see an URL in this format: https://signal.art/addstickers/#pack_id=XXX&pack_key=YYY, where XXX is the pack_id and YYY is the pack_key.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Success 204
// @Failure 400 {object} Error
// @Param data body AddStickerPackRequest true "Request"
// @Router /v1/sticker-packs/{number} [post]
func (a *Api) AddStickerPack(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req AddStickerPackRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.AddStickerPack(number, req.PackId, req.PackKey)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(201)
}

// @Summary List Contacts
// @Tags Contacts
// @Description List all contacts for the given number.
// @Produce  json
// @Success 200 {object} []client.ListContactsResponse
// @Param number path string true "Registered Phone Number"
// @Router /v1/contacts/{number} [get]
func (a *Api) ListContacts(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	contacts, err := a.signalClient.ListContacts(number)

	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.JSON(200, contacts)
}

type PluginInputData struct {
	Params  map[string]string
	Payload string
}

type PluginOutputData struct {
	payload string
	httpStatusCode int
}

func (p *PluginOutputData) SetPayload(payload string) {
	p.payload = payload
}

func (p *PluginOutputData) Payload() string {
	return p.payload
}

func (p *PluginOutputData) SetHttpStatusCode(httpStatusCode int) {
	p.httpStatusCode = httpStatusCode
}

func (p *PluginOutputData) HttpStatusCode() int {
	return p.httpStatusCode
}

func (a *Api) ExecutePlugin(c *gin.Context, pluginConfig utils.PluginConfig) {
	jsonData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid input data"})
		log.Error(err.Error())
		return
	}

	pluginInputData := &PluginInputData{
		Params: make(map[string]string),
		Payload: string(jsonData),
	}

	pluginOutputData := &PluginOutputData{
		payload: "",
		httpStatusCode: 200,
	}

	parts := strings.Split(pluginConfig.Endpoint, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ":") {
			paramName := strings.TrimPrefix(part, ":")
			pluginInputData.Params[paramName] = c.Param(paramName)
		}
	}

	l := lua.NewState()
	l.SetGlobal("pluginInputData", luar.New(l, pluginInputData))
	l.SetGlobal("pluginOutputData", luar.New(l, pluginOutputData))
	l.PreloadModule("http", gluahttp.NewHttpModule(&http.Client{}).Loader)
	luajson.Preload(l)
	defer l.Close()
	if err := l.DoFile(pluginConfig.ScriptPath); err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.JSON(pluginOutputData.HttpStatusCode(), pluginOutputData.Payload())
}
