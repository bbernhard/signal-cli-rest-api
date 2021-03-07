package api

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cyphar/filepath-securejoin"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	uuid "github.com/gofrs/uuid"
	"github.com/h2non/filetype"
	log "github.com/sirupsen/logrus"
	qrcode "github.com/skip2/go-qrcode"
	utils "github.com/bbernhard/signal-cli-rest-api/utils"
)

const signalCliV2GroupError = "Cannot create a V2 group as self does not have a versioned profile"

const groupPrefix = "group."

type GroupEntry struct {
	Name            string   `json:"name"`
	Id              string   `json:"id"`
	InternalId      string   `json:"internal_id"`
	Members         []string `json:"members"`
	Blocked         bool     `json:"blocked"`
	PendingInvites  []string `json:"pending_invites"`
	PendingRequests []string `json:"pending_requests"`
	InviteLink      string   `json:"invite_link"`
}

type LoggingConfiguration struct {
	Level            string   `json:"Level"`
}

type Configuration struct {
	Logging            LoggingConfiguration   `json:"logging"`
}

type SignalCliGroupEntry struct {
	Name              string   `json:"name"`
	Id                string   `json:"id"`
	IsMember          bool     `json:"isMember"`
	IsBlocked         bool     `json:"isBlocked"`
	Members           []string `json:"members"`
	PendingMembers    []string `json:"pendingMembers"`
	RequestingMembers []string `json:"requestingMembers"`
	GroupInviteLink   string   `json:"groupInviteLink"`
}

type IdentityEntry struct {
	Number       string `json:"number"`
	Status       string `json:"status"`
	Fingerprint  string `json:"fingerprint"`
	Added        string `json:"added"`
	SafetyNumber string `json:"safety_number"`
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

type About struct {
	SupportedApiVersions []string `json:"versions"`
	BuildNr              int      `json:"build"`
}

type CreateGroup struct {
	Id string `json:"id"`
}

type UpdateProfileRequest struct {
	Name         string `json:"name"`
	Base64Avatar string `json:"base64_avatar"`
}

type TrustIdentityRequest struct {
	VerifiedSafetyNumber string `json:"verified_safety_number"`
}

func convertInternalGroupIdToGroupId(internalId string) string {
	return groupPrefix + base64.StdEncoding.EncodeToString([]byte(internalId))
}

func convertGroupIdToInternalGroupId(id string) (string, error) {
	groupIdWithoutPrefix := strings.TrimPrefix(id, groupPrefix)
	internalGroupId, err := base64.StdEncoding.DecodeString(groupIdWithoutPrefix)
	if err != nil {
		return "", errors.New("Invalid group id")
	}

	return string(internalGroupId), err
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

func getContainerId() (string, error) {
	data, err := ioutil.ReadFile("/proc/1/cpuset")
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return "", errors.New("Couldn't get docker container id (empty)")
	}
	containerId := strings.Replace(lines[0], "/docker/", "", -1)
	return containerId, nil
}

func send(c *gin.Context, attachmentTmpDir string, signalCliConfig string, number string, message string,
	recipients []string, base64Attachments []string, isGroup bool) {
	cmd := []string{"--config", signalCliConfig, "-u", number, "send"}

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

	_, err := runSignalCli(true, cmd, message)
	if err != nil {
		cleanupTmpFiles(attachmentTmpPaths)
		if strings.Contains(err.Error(), signalCliV2GroupError) {
			c.JSON(400, Error{Msg: "Cannot send message to group - please first update your profile."})
		} else {
			c.JSON(400, Error{Msg: err.Error()})
		}
		return
	}

	cleanupTmpFiles(attachmentTmpPaths)
	c.Writer.WriteHeader(201)
}

func parseWhitespaceDelimitedKeyValueStringList(in string, keys []string) []map[string]string {
	l := []map[string]string{}
	lines := strings.Split(in, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		m := make(map[string]string)

		temp := line
		for i, key := range keys {
			if i == 0 {
				continue
			}

			idx := strings.Index(temp, " "+key+": ")
			pair := temp[:idx]
			value := strings.TrimPrefix(pair, key+": ")
			temp = strings.TrimLeft(temp[idx:], " "+key+": ")

			m[keys[i-1]] = value
		}
		m[keys[len(keys)-1]] = temp

		l = append(l, m)
	}
	return l
}

func getGroups(number string, signalCliConfig string) ([]GroupEntry, error) {
	groupEntries := []GroupEntry{}

	out, err := runSignalCli(true, []string{"--config", signalCliConfig, "--output", "json", "-u", number, "listGroups", "-d"}, "")
	if err != nil {
		return groupEntries, err
	}

	var signalCliGroupEntries []SignalCliGroupEntry

	err = json.Unmarshal([]byte(out), &signalCliGroupEntries)
	if err != nil {
		return groupEntries, err
	}

	for _, signalCliGroupEntry := range signalCliGroupEntries {
		var groupEntry GroupEntry
		groupEntry.InternalId = signalCliGroupEntry.Id
		groupEntry.Name = signalCliGroupEntry.Name
		groupEntry.Id = convertInternalGroupIdToGroupId(signalCliGroupEntry.Id)
		groupEntry.Blocked = signalCliGroupEntry.IsBlocked
		groupEntry.Members = signalCliGroupEntry.Members
		groupEntry.PendingRequests = signalCliGroupEntry.PendingMembers
		groupEntry.PendingInvites = signalCliGroupEntry.RequestingMembers
		groupEntry.InviteLink = signalCliGroupEntry.GroupInviteLink

		groupEntries = append(groupEntries, groupEntry)
	}

	return groupEntries, nil
}

func runSignalCli(wait bool, args []string, stdin string) (string, error) {
	containerId, err := getContainerId()

	log.Debug("If you want to run this command manually, run the following steps on your host system:")
	if err == nil {
		log.Debug("*) docker exec -it ", containerId, " /bin/bash")
	} else {
		log.Debug("*) docker exec -it <container id> /bin/bash")
	}

	signalCliBinary := "signal-cli"
	if utils.GetEnv("USE_NATIVE", "0") == "1" {
		if utils.GetEnv("SUPPORTS_NATIVE", "0") == "1" {
			signalCliBinary = "signal-cli-native"
		} else {
			log.Error("signal-cli-native is not support on this system...falling back to signal-cli")
			signalCliBinary = "signal-cli"
		}
	}

	log.Debug("*) su signal-api")
	log.Debug("*) ", signalCliBinary, " ", strings.Join(args, " "))

	cmd := exec.Command(signalCliBinary, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
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
	avatarTmpDir     string
}

func NewApi(signalCliConfig string, attachmentTmpDir string, avatarTmpDir string) *Api {
	return &Api{
		signalCliConfig:  signalCliConfig,
		attachmentTmpDir: attachmentTmpDir,
		avatarTmpDir:     avatarTmpDir,
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

	command := []string{"--config", a.signalCliConfig, "-u", number, "register"}

	if req.UseVoice == true {
		command = append(command, "--voice")
	}

	if req.Captcha != "" {
		command = append(command, []string{"--captcha", req.Captcha}...)
	}

	_, err := runSignalCli(true, command, "")
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

	cmd := []string{"--config", a.signalCliConfig, "-u", number, "verify", token}
	if pin != "" {
		cmd = append(cmd, "--pin")
		cmd = append(cmd, pin)
	}

	_, err := runSignalCli(true, cmd, "")
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
// @Param timeout query string false "Receive timeout in seconds (default: 1)"
// @Router /v1/receive/{number} [get]
func (a *Api) Receive(c *gin.Context) {
	number := c.Param("number")

	timeout := c.DefaultQuery("timeout", "1")

	command := []string{"--config", a.signalCliConfig, "--output", "json", "-u", number, "receive", "-t", timeout}
	out, err := runSignalCli(true, command, "")
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

	out, err := runSignalCli(true, cmd, "")
	if err != nil {
		if strings.Contains(err.Error(), signalCliV2GroupError) {
			c.JSON(400, Error{Msg: "Cannot create group - please first update your profile."})
		} else {
			c.JSON(400, Error{Msg: err.Error()})
		}
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

// @Summary List a Signal Group.
// @Tags Groups
// @Description List a specific Signal Group.
// @Accept  json
// @Produce  json
// @Success 200 {object} GroupEntry
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid} [get]
func (a *Api) GetGroup(c *gin.Context) {
	number := c.Param("number")
	groupId := c.Param("groupid")

	groups, err := getGroups(number, a.signalCliConfig)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	for _, group := range groups {
		if group.Id == groupId {
			c.JSON(200, group)
			return
		}
	}

	c.JSON(404, Error{Msg: "No group with that id found"})
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
		c.JSON(400, gin.H{"error": "Please specify a group id"})
		return
	}

	groupId, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(base64EncodedGroupId, groupPrefix))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid group id"})
		return
	}

	_, err = runSignalCli(true, []string{"--config", a.signalCliConfig, "-u", number, "quitGroup", "-g", string(groupId)}, "")
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
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
		c.JSON(400, gin.H{"error": "Please provide a name for the device"})
		return
	}

	command := []string{"--config", a.signalCliConfig, "link", "-n", deviceName}

	tsdeviceLink, err := runSignalCli(false, command, "")
	if err != nil {
		log.Error("Couldn't create QR code: ", err.Error())
		c.JSON(400, Error{Msg: "Couldn't create QR code: " + err.Error()})
		return
	}

	q, err := qrcode.New(string(tsdeviceLink), qrcode.Medium)
	if err != nil {
		log.Error("Couldn't create QR code: ", err.Error())
		c.JSON(400, Error{Msg: "Couldn't create QR code: " + err.Error()})
		return
	}

	q.DisableBorder = false
	var png []byte
	png, err = q.PNG(256)
	if err != nil {
		log.Error("Couldn't create QR code: ", err.Error())
		c.JSON(400, Error{Msg: "Couldn't create QR code: " + err.Error()})
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
	files := []string{}
	err := filepath.Walk(a.signalCliConfig+"/attachments/", func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		files = append(files, filepath.Base(path))
		return nil
	})

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
	path, err := securejoin.SecureJoin(a.signalCliConfig+"/attachments/", attachment)
	if err != nil {
		c.JSON(400, Error{Msg: "Please provide a valid attachment name"})
		return
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		c.JSON(404, Error{Msg: "No attachment with that name found"})
		return
	}
	err = os.Remove(path)
	if err != nil {
		c.JSON(500, Error{Msg: "Couldn't delete attachment - please try again later"})
		return
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
	path, err := securejoin.SecureJoin(a.signalCliConfig+"/attachments/", attachment)
	if err != nil {
		c.JSON(400, Error{Msg: "Please provide a valid attachment name"})
		return
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		c.JSON(404, Error{Msg: "No attachment with that name found"})
		return
	}

	imgBytes, err := ioutil.ReadFile(path)
	if err != nil {
		c.JSON(500, Error{Msg: "Couldn't read attachment - please try again later"})
		return
	}

	mime, err := mimetype.DetectReader(bytes.NewReader(imgBytes))
	if err != nil {
		c.JSON(500, Error{Msg: "Couldn't detect MIME type for attachment"})
		return
	}

	c.Writer.Header().Set("Content-Type", mime.String())
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(imgBytes)))
	_, err = c.Writer.Write(imgBytes)
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
	cmd := []string{"--config", a.signalCliConfig, "-u", number, "updateProfile", "--name", req.Name}

	avatarTmpPaths := []string{}
	if req.Base64Avatar == "" {
		cmd = append(cmd, "--remove-avatar")
	} else {
		u, err := uuid.NewV4()
		if err != nil {
			c.JSON(400, Error{Msg: err.Error()})
			return
		}

		avatarBytes, err := base64.StdEncoding.DecodeString(req.Base64Avatar)
		if err != nil {
			c.JSON(400, Error{Msg: "Couldn't decode base64 encoded avatar"})
			return
		}

		fType, err := filetype.Get(avatarBytes)
		if err != nil {
			c.JSON(400, Error{Msg: err.Error()})
			return
		}

		avatarTmpPath := a.avatarTmpDir + u.String() + "." + fType.Extension

		f, err := os.Create(avatarTmpPath)
		if err != nil {
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
		defer f.Close()

		if _, err := f.Write(avatarBytes); err != nil {
			cleanupTmpFiles(avatarTmpPaths)
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
		if err := f.Sync(); err != nil {
			cleanupTmpFiles(avatarTmpPaths)
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
		f.Close()

		cmd = append(cmd, []string{"--avatar", avatarTmpPath}...)
		avatarTmpPaths = append(avatarTmpPaths, avatarTmpPath)
	}

	_, err = runSignalCli(true, cmd, "")
	if err != nil {
		cleanupTmpFiles(avatarTmpPaths)
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	cleanupTmpFiles(avatarTmpPaths)
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
// @Success 200 {object} []IdentityEntry
// @Param number path string true "Registered Phone Number"
// @Router /v1/identities/{number} [get]
func (a *Api) ListIdentities(c *gin.Context) {
	number := c.Param("number")

	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	out, err := runSignalCli(true, []string{"--config", a.signalCliConfig, "-u", number, "listIdentities"}, "")
	if err != nil {
		c.JSON(500, Error{Msg: err.Error()})
		return
	}

	identityEntries := []IdentityEntry{}
	keyValuePairs := parseWhitespaceDelimitedKeyValueStringList(out, []string{"NumberAndTrustStatus", "Added", "Fingerprint", "Safety Number"})
	for _, keyValuePair := range keyValuePairs {
		numberAndTrustStatus := keyValuePair["NumberAndTrustStatus"]
		numberAndTrustStatusSplitted := strings.Split(numberAndTrustStatus, ":")

		identityEntry := IdentityEntry{Number: strings.Trim(numberAndTrustStatusSplitted[0], " "),
			Status:       strings.Trim(numberAndTrustStatusSplitted[1], " "),
			Added:        keyValuePair["Added"],
			Fingerprint:  strings.Trim(keyValuePair["Fingerprint"], " "),
			SafetyNumber: strings.Trim(keyValuePair["Safety Number"], " "),
		}
		identityEntries = append(identityEntries, identityEntry)
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

	cmd := []string{"--config", a.signalCliConfig, "-u", number, "trust", numberToTrust, "--verified-safety-number", req.VerifiedSafetyNumber}
	_, err = runSignalCli(true, cmd, "")
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
	internalGroupId, err := convertGroupIdToInternalGroupId(groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	_, err = runSignalCli(true, []string{"--config", a.signalCliConfig, "-u", number, "block", "-g", internalGroupId}, "")
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
	internalGroupId, err := convertGroupIdToInternalGroupId(groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	_, err = runSignalCli(true, []string{"--config", a.signalCliConfig, "-u", number, "updateGroup", "-g", internalGroupId}, "")
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
	internalGroupId, err := convertGroupIdToInternalGroupId(groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	_, err = runSignalCli(true, []string{"--config", a.signalCliConfig, "-u", number, "quitGroup", "-g", internalGroupId}, "")
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
