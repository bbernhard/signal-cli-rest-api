package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/gorilla/websocket"

	"github.com/bbernhard/signal-cli-rest-api/client"
	utils "github.com/bbernhard/signal-cli-rest-api/utils"
)

const (
	// Time allowed to write the file to the client.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the client.
	pongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

type GroupPermissions struct {
	AddMembers string `json:"add_members" enums:"only-admins,every-member"`
	EditGroup string `json:"edit_group" enums:"only-admins,every-member"`
}

type CreateGroupRequest struct {
	Name    string   `json:"name"`
	Members []string `json:"members"`
	Description string `json:"description"`
	Permissions GroupPermissions `json:"permissions"`
	GroupLinkState string `json:"group_link" enums:"disabled,enabled,enabled-with-approval"`
}

type LoggingConfiguration struct {
	Level            string   `json:"Level"`
}

type Configuration struct {
	Logging            LoggingConfiguration   `json:"logging"`
}

type RegisterNumberRequest struct {
	UseVoice bool   `json:"use_voice"`
	Captcha  string `json:"captcha"`
}

type VerifyNumberSettings struct {
	Pin string `json:"pin"`
}

type SendMessageV1 struct {
	Number           string   `json:"number"`
	Recipients       []string `json:"recipients"`
	Message          string   `json:"message"`
	Base64Attachment string   `json:"base64_attachment"`
	IsGroup          bool     `json:"is_group"`
}

type SendMessageV2 struct {
	Number            string   `json:"number"`
	Recipients        []string `json:"recipients"`
	Message           string   `json:"message"`
	Base64Attachments []string `json:"base64_attachments"`
}

type Error struct {
	Msg string `json:"error"`
}



type CreateGroupResponse struct {
	Id string `json:"id"`
}

type UpdateProfileRequest struct {
	Name         string `json:"name"`
	Base64Avatar string `json:"base64_avatar"`
}

type TrustIdentityRequest struct {
	VerifiedSafetyNumber string `json:"verified_safety_number"`
}

type SendMessageResponse struct {
	Timestamp string `json:"timestamp"`
}

var connectionUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Api struct {
	signalClient  *client.SignalClient
}

func NewApi(signalClient *client.SignalClient) *Api {
	return &Api{
		signalClient:  signalClient,
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
	number := c.Param("number")

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

	err := a.signalClient.RegisterNumber(number, req.UseVoice, req.Captcha)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Writer.WriteHeader(201)
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
	number := c.Param("number")
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

	err := a.signalClient.VerifyRegisteredNumber(number, token, pin)
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
// @Description Send a signal message
// @Accept  json
// @Produce  json
// @Success 201 {object} SendMessageResponse
// @Failure 400 {object} Error
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

	if len(req.Recipients) == 0 {
		c.JSON(400, gin.H{"error": "Couldn't process request - please provide at least one recipient"})
		return
	}

	if req.Number == "" {
		c.JSON(400, gin.H{"error": "Couldn't process request - please provide a valid number"})
		return
	}

	timestamps, err := a.signalClient.SendV2(req.Number, req.Message, req.Recipients, req.Base64Attachments)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.JSON(201, SendMessageResponse{Timestamp: strconv.FormatInt((*timestamps)[0].Timestamp, 10)})
}


func (a *Api) handleSignalReceive(ws *websocket.Conn, number string) {
	for {
		data, err := a.signalClient.Receive(number, 0)
		if err == nil {
			err = ws.WriteMessage(websocket.TextMessage, []byte(data))
			if err != nil {
				log.Error("Couldn't write message: " + err.Error())
				return
			}
		} else {
			errorMsg := Error{Msg: err.Error()}
			errorMsgBytes, err := json.Marshal(errorMsg)
			if err != nil {
				log.Error("Couldn't serialize error message: " + err.Error())
				return
			}
			err = ws.WriteMessage(websocket.TextMessage, errorMsgBytes)
			if err != nil {
				log.Error("Couldn't write message: " + err.Error())
				return
			}
		}
	}
}

func wsPong(ws *websocket.Conn) {
	ws.SetReadLimit(512)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
}

func wsPing(ws *websocket.Conn) {
	pingTicker := time.NewTicker(pingPeriod)
	for {
		select {
		case <-pingTicker.C:
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// @Summary Receive Signal Messages.
// @Tags Messages
// @Description Receives Signal Messages from the Signal Network.
// @Accept  json
// @Produce  json
// @Success 200 {object} []string
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param timeout query string false "Receive timeout in seconds (default: 1)"
// @Router /v1/receive/{number} [get]
func (a *Api) Receive(c *gin.Context) {
	number := c.Param("number")

	if a.signalClient.GetSignalCliMode() == client.JsonRpc {
		ws, err := connectionUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
		defer ws.Close()
		go a.handleSignalReceive(ws, number)
		go wsPing(ws)
		wsPong(ws)
	} else {
		timeout := c.DefaultQuery("timeout", "1")
		timeoutInt, err := strconv.ParseInt(timeout, 10, 32)
		if err != nil {
			c.JSON(400, Error{Msg: "Couldn't process request - timeout needs to be numeric!"})
			return
		}

		jsonStr, err := a.signalClient.Receive(number, timeoutInt)
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
	number := c.Param("number")

	var req CreateGroupRequest
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	if req.Permissions.AddMembers != "" && !utils.StringInSlice(req.Permissions.AddMembers, []string{"every-member", "only-admins"}) {
		c.JSON(400, Error{Msg: "Invalid add members permission provided - only 'every-member' and 'only-admins' allowed!"})
		return
	}

	if req.Permissions.EditGroup != "" && !utils.StringInSlice(req.Permissions.EditGroup, []string{"every-member", "only-admins"}) {
		c.JSON(400, Error{Msg: "Invalid edit group permissions provided - only 'every-member' and 'only-admins' allowed!"})
		return
	}

	if req.GroupLinkState != "" && !utils.StringInSlice(req.GroupLinkState, []string{"enabled", "enabled-with-approval", "disabled"}) {
		c.JSON(400, Error{Msg: "Invalid group link provided - only 'enabled', 'enabled-with-approval' and 'disabled' allowed!" })
		return
	}

	editGroupPermission := client.DefaultGroupPermission
	addMembersPermission := client.DefaultGroupPermission
	groupLinkState := client.DefaultGroupLinkState

	groupId, err := a.signalClient.CreateGroup(number, req.Name, req.Members, req.Description, editGroupPermission, addMembersPermission, groupLinkState)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.JSON(201, CreateGroupResponse{Id: groupId})
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
	number := c.Param("number")

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
	number := c.Param("number")
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
	number := c.Param("number")

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
// @Failure 400 {object} Error
// @Router /v1/qrcodelink [get]
func (a *Api) GetQrCodeLink(c *gin.Context) {
	deviceName := c.Query("device_name")

	if deviceName == "" {
		c.JSON(400, Error{Msg: "Please provide a name for the device"})
		return
	}

	png, err := a.signalClient.GetQrCodeLink(deviceName)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Data(200, "image/png", png)
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
	number := c.Param("number")

	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req UpdateProfileRequest
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	if req.Name == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - profile name missing"})
		return
	}

	err = a.signalClient.UpdateProfile(number, req.Name, req.Base64Avatar)
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
	number := c.Param("number")

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
// @Description Trust an identity.
// @Produce  json
// @Success 204 {string} OK
// @Param data body TrustIdentityRequest true "Input Data"
// @Param number path string true "Registered Phone Number"
// @Param numberToTrust path string true "Number To Trust"
// @Router /v1/identities/{number}/trust/{numberToTrust} [put]
func (a *Api) TrustIdentity(c *gin.Context) {
	number := c.Param("number")

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
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	if req.VerifiedSafetyNumber == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - verified safety number missing"})
		return
	}

	err = a.signalClient.TrustIdentity(number, numberToTrust, req.VerifiedSafetyNumber)
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
// @Success 201 {string} string "OK"
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
// @Success 200 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/block [post]
func (a *Api) BlockGroup(c *gin.Context) {
	number := c.Param("number")
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
// @Success 200 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/join [post]
func (a *Api) JoinGroup(c *gin.Context) {
	number := c.Param("number")
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

// @Summary Quit a Signal Group.
// @Tags Groups
// @Description Quit the specified Signal Group.
// @Accept  json
// @Produce  json
// @Success 200 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/quit [post]
func (a *Api) QuitGroup(c *gin.Context) {
	number := c.Param("number")
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
