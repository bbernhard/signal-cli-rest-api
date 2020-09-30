package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	commands "github.com/bbernhard/signal-cli-rest-api/commands"
	datastructures "github.com/bbernhard/signal-cli-rest-api/datastructures"
	"strings"
)


type Api struct {
	signalCliConfig  string
	attachmentTmpDir string
}

func NewApi(signalCliConfig string, attachmentTmpDir string) *Api {
	return &Api{
		signalCliConfig:  signalCliConfig,
		attachmentTmpDir: attachmentTmpDir,
	}
}

// @Summary Lists general information about the API
// @Tags General
// @Description Returns the supported API versions and the internal build nr
// @Produce  json
// @Success 200 {object} datastructures.About
// @Router /v1/about [get]
func (a *Api) About(c *gin.Context) {

	about := datastructures.About{SupportedApiVersions: []string{"v1", "v2"}, BuildNr: 2}
	c.JSON(200, about)
}

// @Summary Register a phone number.
// @Tags Devices
// @Description Register a phone number with the signal network.
// @Accept  json
// @Produce  json
// @Success 201
// @Failure 400 {object} datastructures.Error
// @Param number path string true "Registered Phone Number"
// @Router /v1/register/{number} [post]
func (a *Api) RegisterNumber(c *gin.Context) {
	number := c.Param("number")

	var req datastructures.RegisterNumberRequest

	buf := new(bytes.Buffer)
	buf.ReadFrom(c.Request.Body)
	if buf.String() != "" {
		err := json.Unmarshal(buf.Bytes(), &req)
		if err != nil {
			log.Error("Couldn't register number: ", err.Error())
			c.JSON(400, datastructures.Error{Msg: "Couldn't process request - invalid request."})
			return
		}
	} else {
		req.UseVoice = false
	}

	if number == "" {
		c.JSON(400, gin.H{"error": "Please provide a number"})
		return
	}

	err := commands.RegisterNumber(a.signalCliConfig, number, req.UseVoice)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, nil)
}

// @Summary Verify a registered phone number.
// @Tags Devices
// @Description Verify a registered phone number with the signal network.
// @Accept  json
// @Produce  json
// @Success 201 {string} string "OK"
// @Failure 400 {object} datastructures.Error
// @Param number path string true "Registered Phone Number"
// @Param data body datastructures.VerifyNumberSettings true "Additional Settings"
// @Param token path string true "Verification Code"
// @Router /v1/register/{number}/verify/{token} [post]
func (a *Api) VerifyRegisteredNumber(c *gin.Context) {
	number := c.Param("number")
	token := c.Param("token")

	pin := ""
	var req datastructures.VerifyNumberSettings
	buf := new(bytes.Buffer)
	buf.ReadFrom(c.Request.Body)
	if buf.String() != "" {
		err := json.Unmarshal(buf.Bytes(), &req)
		if err != nil {
			log.Error("Couldn't verify number: ", err.Error())
			c.JSON(400, datastructures.Error{Msg: "Couldn't process request - invalid request."})
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

	err := commands.VerifyRegisteredNumber(a.signalCliConfig, number, token, pin)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, nil)
}

// @Summary Send a signal message.
// @Tags Messages
// @Description Send a signal message
// @Accept  json
// @Produce  json
// @Success 201 {string} string "OK"
// @Failure 400 {object} datastructures.Error
// @Param data body datastructures.SendMessageV1 true "Input Data"
// @Router /v1/send [post]
// @Deprecated
func (a *Api) Send(c *gin.Context) {

	var req datastructures.SendMessageV1
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, gin.H{"error": "Couldn't process request - invalid request"})
		return
	}

	base64Attachments := []string{}
	if req.Base64Attachment != "" {
		base64Attachments = append(base64Attachments, req.Base64Attachment)
	}

	commands.SendMessage(c, a.signalCliConfig, a.signalCliConfig, req.Number, req.Message, req.Recipients, base64Attachments, req.IsGroup)
}

// @Summary Send a signal message.
// @Tags Messages
// @Description Send a signal message
// @Accept  json
// @Produce  json
// @Success 201 {string} string "OK"
// @Failure 400 {object} datastructures.Error
// @Param data body datastructures.SendMessageV2 true "Input Data"
// @Router /v2/send [post]
func (a *Api) SendV2(c *gin.Context) {
	var req datastructures.SendMessageV2
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

	groups := []string{}
	recipients := []string{}

	for _, recipient := range req.Recipients {
		if strings.HasPrefix(recipient, datastructures.GroupPrefix) {
			groups = append(groups, strings.TrimPrefix(recipient, datastructures.GroupPrefix))
		} else {
			recipients = append(recipients, recipient)
		}
	}

	if len(recipients) > 0 && len(groups) > 0 {
		c.JSON(400, gin.H{"error": "Signal Messenger Groups and phone numbers cannot be specified together in one request! Please split them up into multiple REST API calls."})
		return
	}

	if len(groups) > 1 {
		c.JSON(400, gin.H{"error": "A signal message cannot be sent to more than one group at once! Please use multiple REST API calls for that."})
		return
	}

	for _, group := range groups {
		commands.SendMessage(c, a.attachmentTmpDir, a.signalCliConfig, req.Number, req.Message, []string{group}, req.Base64Attachments, true)
	}

	if len(recipients) > 0 {
		commands.SendMessage(c, a.attachmentTmpDir, a.signalCliConfig, req.Number, req.Message, recipients, req.Base64Attachments, false)
	}
}

// @Summary Receive Signal Messages.
// @Tags Messages
// @Description Receives Signal Messages from the Signal Network.
// @Accept  json
// @Produce  json
// @Success 200 {object} []string
// @Failure 400 {object} datastructures.Error
// @Param number path string true "Registered Phone Number"
// @Router /v1/receive/{number} [get]
func (a *Api) Receive(c *gin.Context) {
	number := c.Param("number")

	res, err := commands.Receive(a.signalCliConfig, number)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.String(200, res)
}

// @Summary Create a new Signal Group.
// @Tags Groups
// @Description Create a new Signal Group with the specified members.
// @Accept  json
// @Produce  json
// @Success 201 {object} datastructures.CreateGroup
// @Failure 400 {object} datastructures.Error
// @Param number path string true "Registered Phone Number"
// @Router /v1/groups/{number} [post]
func (a *Api) CreateGroup(c *gin.Context) {
	number := c.Param("number")

	type Request struct {
		Name    string   `json:"name"`
		Members []string `json:"members"`
	}

	var req Request
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, gin.H{"error": "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	internalGroupId, err := commands.CreateGroup(a.signalCliConfig, number, req.Name, req.Members)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, datastructures.CreateGroup{Id: commands.ConvertInternalGroupIdToGroupId(internalGroupId)})
}

// @Summary List all Signal Groups.
// @Tags Groups
// @Description List all Signal Groups.
// @Accept  json
// @Produce  json
// @Success 200 {object} []datastructures.GroupEntry
// @Failure 400 {object} datastructures.Error
// @Param number path string true "Registered Phone Number"
// @Router /v1/groups/{number} [get]
func (a *Api) GetGroups(c *gin.Context) {
	number := c.Param("number")

	groups, err := commands.GetGroups(a.signalCliConfig, number)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, groups)
}

// @Summary Delete a Signal Group.
// @Tags Groups
// @Description Delete a Signal Group.
// @Accept  json
// @Produce  json
// @Success 200 {string} string "OK"
// @Failure 400 {object} datastructures.Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group Id"
// @Router /v1/groups/{number}/{groupid} [delete]
func (a *Api) DeleteGroup(c *gin.Context) {
	base64EncodedGroupId := c.Param("groupid")
	number := c.Param("number")

	if base64EncodedGroupId == "" {
		c.JSON(400, gin.H{"error": "Please specify a group id"})
		return
	}

	groupId, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(base64EncodedGroupId, datastructures.GroupPrefix))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid group id"})
		return
	}

	err = commands.DeleteGroup(a.signalCliConfig, number, string(groupId))
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(204, nil)
}

// @Summary Link device and generate QR code.
// @Tags Devices
// @Description test
// @Produce  json
// @Success 200 {string} string	"Image"
// @Router /v1/qrcodelink [get]
func (a *Api) GetQrCodeLink(c *gin.Context) {
	deviceName := c.Query("device_name")

	if deviceName == "" {
		c.JSON(400, gin.H{"error": "Please provide a name for the device"})
		return
	}

	png, err := commands.GetQrCodeLink(a.signalCliConfig, deviceName)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.Data(200, "image/png", png)
}
