package api

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	uuid "github.com/gofrs/uuid"
	"github.com/h2non/filetype"
	log "github.com/sirupsen/logrus"
	qrcode "github.com/skip2/go-qrcode"
	"os"
	"os/exec"
	"strings"
	"time"
)

const groupPrefix = "group."

type GroupEntry struct {
	Name       string   `json:"name"`
	Id         string   `json:"id"`
	InternalId string   `json:"internal_id"`
	Members    []string `json:"members"`
	Active     bool     `json:"active"`
	Blocked    bool     `json:"blocked"`
}

type RegisterNumberRequest struct {
	UseVoice bool `json:"use_voice"`
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

type About struct {
	SupportedApiVersions []string `json:"versions"`
	BuildNr              int      `json:"build"`
}

type CreateGroup struct {
	Id string `json:"id"`
}

func convertInternalGroupIdToGroupId(internalId string) string {
	return groupPrefix + base64.StdEncoding.EncodeToString([]byte(internalId))
}

func getStringInBetween(str string, start string, end string) (result string) {
	i := strings.Index(str, start)
	if i == -1 {
		return
	}
	i += len(start)
	j := strings.Index(str[i:], end)
	if j == -1 {
		return
	}
	return str[i : i+j]
}

func cleanupTmpFiles(paths []string) {
	for _, path := range paths {
		os.Remove(path)
	}
}

func send(c *gin.Context, attachmentTmpDir string, signalCliConfig string, number string, message string,
	recipients []string, base64Attachments []string, isGroup bool) {
	cmd := []string{"--config", signalCliConfig, "-u", number, "send", "-m", message}

	if len(recipients) == 0 {
		c.JSON(400, gin.H{"error": "Please specify at least one recipient"})
		return
	}

	if !isGroup {
		cmd = append(cmd, recipients...)
	} else {
		if len(recipients) > 1 {
			c.JSON(400, gin.H{"error": "More than one recipient is currently not allowed"})
			return
		}

		groupId, err := base64.StdEncoding.DecodeString(recipients[0])
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid group id"})
			return
		}

		cmd = append(cmd, []string{"-g", string(groupId)}...)
	}

	attachmentTmpPaths := []string{}
	for _, base64Attachment := range base64Attachments {
		u, err := uuid.NewV4()
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		dec, err := base64.StdEncoding.DecodeString(base64Attachment)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		fType, err := filetype.Get(dec)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		attachmentTmpPath := attachmentTmpDir + u.String() + "." + fType.Extension
		attachmentTmpPaths = append(attachmentTmpPaths, attachmentTmpPath)

		f, err := os.Create(attachmentTmpPath)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		defer f.Close()

		if _, err := f.Write(dec); err != nil {
			cleanupTmpFiles(attachmentTmpPaths)
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := f.Sync(); err != nil {
			cleanupTmpFiles(attachmentTmpPaths)
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		f.Close()
	}

	if len(attachmentTmpPaths) > 0 {
		cmd = append(cmd, "-a")
		cmd = append(cmd, attachmentTmpPaths...)
	}

	_, err := runSignalCli(true, cmd)
	if err != nil {
		cleanupTmpFiles(attachmentTmpPaths)
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, nil)
}

func getGroups(number string, signalCliConfig string) ([]GroupEntry, error) {
	groupEntries := []GroupEntry{}

	out, err := runSignalCli(true, []string{"--config", signalCliConfig, "-u", number, "listGroups", "-d"})
	if err != nil {
		return groupEntries, err
	}

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		var groupEntry GroupEntry
		if line == "" {
			continue
		}

		idIdx := strings.Index(line, " Name: ")
		idPair := line[:idIdx]
		groupEntry.InternalId = strings.TrimPrefix(idPair, "Id: ")
		groupEntry.Id = convertInternalGroupIdToGroupId(groupEntry.InternalId)
		lineWithoutId := strings.TrimLeft(line[idIdx:], " ")

		nameIdx := strings.Index(lineWithoutId, " Active: ")
		namePair := lineWithoutId[:nameIdx]
		groupEntry.Name = strings.TrimRight(strings.TrimPrefix(namePair, "Name: "), " ")
		lineWithoutName := strings.TrimLeft(lineWithoutId[nameIdx:], " ")

		activeIdx := strings.Index(lineWithoutName, " Blocked: ")
		activePair := lineWithoutName[:activeIdx]
		active := strings.TrimPrefix(activePair, "Active: ")
		if active == "true" {
			groupEntry.Active = true
		} else {
			groupEntry.Active = false
		}
		lineWithoutActive := strings.TrimLeft(lineWithoutName[activeIdx:], " ")

		blockedIdx := strings.Index(lineWithoutActive, " Members: ")
		blockedPair := lineWithoutActive[:blockedIdx]
		blocked := strings.TrimPrefix(blockedPair, "Blocked: ")
		if blocked == "true" {
			groupEntry.Blocked = true
		} else {
			groupEntry.Blocked = false
		}
		lineWithoutBlocked := strings.TrimLeft(lineWithoutActive[blockedIdx:], " ")

		membersPair := lineWithoutBlocked
		members := strings.TrimPrefix(membersPair, "Members: ")
		trimmedMembers := strings.TrimRight(strings.TrimLeft(members, "["), "]")
		trimmedMembersList := strings.Split(trimmedMembers, ",")
		for _, member := range trimmedMembersList {
			groupEntry.Members = append(groupEntry.Members, strings.Trim(member, " "))
		}

		groupEntries = append(groupEntries, groupEntry)
	}

	return groupEntries, nil
}

func runSignalCli(wait bool, args []string) (string, error) {
	cmd := exec.Command("signal-cli", args...)
	if wait {
		var errBuffer bytes.Buffer
		var outBuffer bytes.Buffer
		cmd.Stderr = &errBuffer
		cmd.Stdout = &outBuffer

		err := cmd.Start()
		if err != nil {
			return "", err
		}

		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()
		select {
		case <-time.After(60 * time.Second):
			err := cmd.Process.Kill()
			if err != nil {
				return "", err
			}
			return "", errors.New("process killed as timeout reached")
		case err := <-done:
			if err != nil {
				return "", errors.New(errBuffer.String())
			}
		}
		return outBuffer.String(), nil
	} else {
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return "", err
		}
		cmd.Start()
		buf := bufio.NewReader(stdout) // Notice that this is not in a loop
		line, _, _ := buf.ReadLine()
		return string(line), nil
	}
}

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
// @Success 200 {object} About
// @Router /v1/about [get]
func (a *Api) About(c *gin.Context) {

	about := About{SupportedApiVersions: []string{"v1", "v2"}, BuildNr: 2}
	c.JSON(200, about)
}

// @Summary Register a phone number.
// @Tags Devices
// @Description Register a phone number with the signal network.
// @Accept  json
// @Produce  json
// @Success 201
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
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
	}

	if number == "" {
		c.JSON(400, gin.H{"error": "Please provide a number"})
		return
	}

	command := []string{"--config", a.signalCliConfig, "-u", number, "register"}

	if req.UseVoice == true {
		command = append(command, "--voice")
	}

	_, err := runSignalCli(true, command)
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
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param data body VerifyNumberSettings true "Additional Settings"
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

	cmd := []string{"--config", a.signalCliConfig, "-u", number, "verify", token}
	if pin != "" {
		cmd = append(cmd, "--pin")
		cmd = append(cmd, pin)
	}

	_, err := runSignalCli(true, cmd)
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
// @Failure 400 {object} Error
// @Param data body SendMessageV1 true "Input Data"
// @Router /v1/send [post]
// @Deprecated
func (a *Api) Send(c *gin.Context) {

	var req SendMessageV1
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, gin.H{"error": "Couldn't process request - invalid request"})
		return
	}

	base64Attachments := []string{}
	if req.Base64Attachment != "" {
		base64Attachments = append(base64Attachments, req.Base64Attachment)
	}

	send(c, a.signalCliConfig, a.signalCliConfig, req.Number, req.Message, req.Recipients, base64Attachments, req.IsGroup)
}

// @Summary Send a signal message.
// @Tags Messages
// @Description Send a signal message
// @Accept  json
// @Produce  json
// @Success 201 {string} string "OK"
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

	groups := []string{}
	recipients := []string{}

	for _, recipient := range req.Recipients {
		if strings.HasPrefix(recipient, groupPrefix) {
			groups = append(groups, strings.TrimPrefix(recipient, groupPrefix))
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
		send(c, a.attachmentTmpDir, a.signalCliConfig, req.Number, req.Message, []string{group}, req.Base64Attachments, true)
	}

	if len(recipients) > 0 {
		send(c, a.attachmentTmpDir, a.signalCliConfig, req.Number, req.Message, recipients, req.Base64Attachments, false)
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
// @Router /v1/receive/{number} [get]
func (a *Api) Receive(c *gin.Context) {
	number := c.Param("number")

	command := []string{"--config", a.signalCliConfig, "-u", number, "receive", "-t", "1", "--json"}
	out, err := runSignalCli(true, command)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	out = strings.Trim(out, "\n")
	lines := strings.Split(out, "\n")

	jsonStr := "["
	for i, line := range lines {
		jsonStr += line
		if i != (len(lines) - 1) {
			jsonStr += ","
		}
	}
	jsonStr += "]"

	c.String(200, jsonStr)
}

// @Summary Create a new Signal Group.
// @Tags Groups
// @Description Create a new Signal Group with the specified members.
// @Accept  json
// @Produce  json
// @Success 201 {object} CreateGroup
// @Failure 400 {object} Error
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

	cmd := []string{"--config", a.signalCliConfig, "-u", number, "updateGroup", "-n", req.Name, "-m"}
	cmd = append(cmd, req.Members...)

	out, err := runSignalCli(true, cmd)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	internalGroupId := getStringInBetween(out, `"`, `"`)
	c.JSON(201, CreateGroup{Id: convertInternalGroupIdToGroupId(internalGroupId)})
}

// @Summary List all Signal Groups.
// @Tags Groups
// @Description List all Signal Groups.
// @Accept  json
// @Produce  json
// @Success 200 {object} []GroupEntry
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Router /v1/groups/{number} [get]
func (a *Api) GetGroups(c *gin.Context) {
	number := c.Param("number")

	groups, err := getGroups(number, a.signalCliConfig)
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
// @Failure 400 {object} Error
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

	groupId, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(base64EncodedGroupId, groupPrefix))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid group id"})
		return
	}

	_, err = runSignalCli(true, []string{"--config", a.signalCliConfig, "-u", number, "quitGroup", "-g", string(groupId)})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
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

	command := []string{"--config", a.signalCliConfig, "link", "-n", deviceName}

	tsdeviceLink, err := runSignalCli(false, command)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	q, err := qrcode.New(string(tsdeviceLink), qrcode.Medium)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
	}

	q.DisableBorder = true
	var png []byte
	png, err = q.PNG(256)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
	}

	c.Data(200, "image/png", png)
}
