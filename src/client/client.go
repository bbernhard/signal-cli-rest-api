package client

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cyphar/filepath-securejoin"
	"github.com/gabriel-vasile/mimetype"
	"github.com/h2non/filetype"
	//"github.com/sourcegraph/jsonrpc2"//"net/rpc/jsonrpc"
	log "github.com/sirupsen/logrus"

	uuid "github.com/gofrs/uuid"
	qrcode "github.com/skip2/go-qrcode"

	utils "github.com/bbernhard/signal-cli-rest-api/utils"
)

const groupPrefix = "group."

const signalCliV2GroupError = "Cannot create a V2 group as self does not have a versioned profile"

type GroupPermission int

const (
	DefaultGroupPermission GroupPermission = iota + 1
	EveryMember
	OnlyAdmins
)

type SignalCliMode int

const (
	Normal SignalCliMode = iota + 1
	Native
	JsonRpc
)

type GroupLinkState int

const (
	DefaultGroupLinkState GroupLinkState = iota + 1
	Enabled
	EnabledWithApproval
	Disabled
)

func (g GroupPermission) String() string {
	return []string{"", "default", "every-member", "only-admins"}[g]
}

func (g GroupLinkState) String() string {
	return []string{"", "enabled", "enabled-with-approval", "disabled"}[g]
}

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

type IdentityEntry struct {
	Number       string `json:"number"`
	Status       string `json:"status"`
	Fingerprint  string `json:"fingerprint"`
	Added        string `json:"added"`
	SafetyNumber string `json:"safety_number"`
}

type SignalCliGroupMember struct {
	Number string `json:"number"`
	Uuid   string `json:"uuid"`
}

type SignalCliGroupEntry struct {
	Name              string                 `json:"name"`
	Id                string                 `json:"id"`
	IsMember          bool                   `json:"isMember"`
	IsBlocked         bool                   `json:"isBlocked"`
	Members           []SignalCliGroupMember `json:"members"`
	PendingMembers    []string               `json:"pendingMembers"`
	RequestingMembers []string               `json:"requestingMembers"`
	GroupInviteLink   string                 `json:"groupInviteLink"`
}

type SignalCliIdentityEntry struct {
	Number                string `json:"number"`
	Uuid                  string `json:"uuid"`
	Fingerprint           string `json:"fingerprint"`
	SafetyNumber          string `json:"safetyNumber"`
	ScannableSafetyNumber string `json:"scannableSafetyNumber"`
	TrustLevel            string `json:"trustLevel"`
	AddedTimestamp        int64  `json:"addedTimestamp"`
}

type SendResponse struct {
	Timestamp int64 `json:"timestamp"`
}

type About struct {
	SupportedApiVersions []string `json:"versions"`
	BuildNr              int      `json:"build"`
}

func cleanupTmpFiles(paths []string) {
	for _, path := range paths {
		os.Remove(path)
	}
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

func runSignalCli(wait bool, args []string, stdin string, signalCliMode SignalCliMode) (string, error) {
	containerId, err := getContainerId()

	log.Debug("If you want to run this command manually, run the following steps on your host system:")
	if err == nil {
		log.Debug("*) docker exec -it ", containerId, " /bin/bash")
	} else {
		log.Debug("*) docker exec -it <container id> /bin/bash")
	}

	signalCliBinary := ""
	if signalCliMode == Normal {
		signalCliBinary = "signal-cli"
	} else if signalCliMode == Native {
		signalCliBinary = "signal-cli-native"
	} else {
		return "", errors.New("Invalid signal-cli mode")
	}

	fullCmd := ""
	if stdin != "" {
		fullCmd += "echo '" + stdin + "' | "
	}
	fullCmd += signalCliBinary + " " + strings.Join(args, " ")

	log.Debug("*) su signal-api")
	log.Debug("*) ", fullCmd)

	cmdTimeout, err := utils.GetIntEnv("SIGNAL_CLI_CMD_TIMEOUT", 120)
	if err != nil {
		log.Error("Env variable 'SIGNAL_CLI_CMD_TIMEOUT' contains an invalid timeout...falling back to default timeout (120 seconds)")
		cmdTimeout = 120
	}

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
		case <-time.After(time.Duration(cmdTimeout) * time.Second):
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

func ConvertGroupIdToInternalGroupId(id string) (string, error) {

	groupIdWithoutPrefix := strings.TrimPrefix(id, groupPrefix)
	internalGroupId, err := base64.StdEncoding.DecodeString(groupIdWithoutPrefix)
	if err != nil {
		return "", errors.New("Invalid group id")
	}

	return string(internalGroupId), err
}

type SignalClient struct {
	signalCliConfig          string
	attachmentTmpDir         string
	avatarTmpDir             string
	signalCliMode            SignalCliMode
	jsonRpc2ClientConfig     *utils.JsonRpc2ClientConfig
	jsonRpc2ClientConfigPath string
	jsonRpc2Clients          map[string]*JsonRpc2Client
}

func NewSignalClient(signalCliConfig string, attachmentTmpDir string, avatarTmpDir string, signalCliMode SignalCliMode,
	jsonRpc2ClientConfigPath string) *SignalClient {
	return &SignalClient{
		signalCliConfig:          signalCliConfig,
		attachmentTmpDir:         attachmentTmpDir,
		avatarTmpDir:             avatarTmpDir,
		signalCliMode:            signalCliMode,
		jsonRpc2ClientConfigPath: jsonRpc2ClientConfigPath,
		jsonRpc2Clients:          make(map[string]*JsonRpc2Client),
	}
}

func (s *SignalClient) Init() error {
	if s.signalCliMode == JsonRpc {
		s.jsonRpc2ClientConfig = utils.NewJsonRpc2ClientConfig()
		err := s.jsonRpc2ClientConfig.Load(s.jsonRpc2ClientConfigPath)
		if err != nil {
			return err
		}

		tcpPortsNumberMapping := s.jsonRpc2ClientConfig.GetTcpPortsForNumbers()
		for number, tcpPort := range tcpPortsNumberMapping {
			s.jsonRpc2Clients[number] = NewJsonRpc2Client()
			err := s.jsonRpc2Clients[number].Dial("127.0.0.1:" + strconv.FormatInt(tcpPort, 10))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *SignalClient) send(number string, message string,
	recipients []string, base64Attachments []string, isGroup bool) (*SendResponse, error) {

	var resp SendResponse

	if len(recipients) == 0 {
		return nil, errors.New("Please specify at least one recipient")
	}

	var groupId string = ""
	if isGroup {
		if len(recipients) > 1 {
			return nil, errors.New("More than one recipient is currently not allowed")
		}

		grpId, err := base64.StdEncoding.DecodeString(recipients[0])
		if err != nil {
			return nil, errors.New("Invalid group id")
		}
		groupId = string(grpId)
	}

	attachmentTmpPaths := []string{}
	for _, base64Attachment := range base64Attachments {
		u, err := uuid.NewV4()
		if err != nil {
			return nil, err
		}

		dec, err := base64.StdEncoding.DecodeString(base64Attachment)
		if err != nil {
			return nil, err
		}

		mimeType := mimetype.Detect(dec)

		attachmentTmpPath := s.attachmentTmpDir + u.String() + mimeType.Extension()
		attachmentTmpPaths = append(attachmentTmpPaths, attachmentTmpPath)

		f, err := os.Create(attachmentTmpPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		if _, err := f.Write(dec); err != nil {
			cleanupTmpFiles(attachmentTmpPaths)
			return nil, err
		}
		if err := f.Sync(); err != nil {
			cleanupTmpFiles(attachmentTmpPaths)
			return nil, err
		}

		f.Close()
	}

	if s.signalCliMode == JsonRpc {
		jsonRpc2Client, err := s.getJsonRpc2Client(number)
		if err != nil {
			return nil, err
		}

		type Request struct {
			Recipients  []string `json:"recipient,omitempty"`
			Message     string   `json:"message"`
			GroupId     string   `json:"group-id,omitempty"`
			Attachments []string `json:"attachment,omitempty"`
		}

		request := Request{Message: message}
		if isGroup {
			request.GroupId = groupId
		} else {
			request.Recipients = recipients
		}
		if len(attachmentTmpPaths) > 0 {
			request.Attachments = append(request.Attachments, attachmentTmpPaths...)
		}

		rawData, err := jsonRpc2Client.getRaw("send", request)
		if err != nil {
			cleanupTmpFiles(attachmentTmpPaths)
			return nil, err
		}

		err = json.Unmarshal([]byte(rawData), &resp)
		if err != nil {
			if strings.Contains(err.Error(), signalCliV2GroupError) {
				return nil, errors.New("Cannot send message to group - please first update your profile.")
			}
			return nil, err
		}
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-u", number, "send"}
		if !isGroup {
			cmd = append(cmd, recipients...)
		} else {
			cmd = append(cmd, []string{"-g", groupId}...)
		}

		if len(attachmentTmpPaths) > 0 {
			cmd = append(cmd, "-a")
			cmd = append(cmd, attachmentTmpPaths...)
		}

		rawData, err := runSignalCli(true, cmd, message, s.signalCliMode)
		if err != nil {
			cleanupTmpFiles(attachmentTmpPaths)
			if strings.Contains(err.Error(), signalCliV2GroupError) {
				return nil, errors.New("Cannot send message to group - please first update your profile.")
			}
			return nil, err
		}
		resp.Timestamp, err = strconv.ParseInt(strings.TrimSuffix(rawData, "\n"), 10, 64)
		if err != nil {
			cleanupTmpFiles(attachmentTmpPaths)
			return nil, err
		}
	}

	cleanupTmpFiles(attachmentTmpPaths)

	return &resp, nil
}

func (s *SignalClient) About() About {
	about := About{SupportedApiVersions: []string{"v1", "v2"}, BuildNr: 2}
	return about
}

func (s *SignalClient) RegisterNumber(number string, useVoice bool, captcha string) error {
	command := []string{"--config", s.signalCliConfig, "-u", number, "register"}

	if useVoice {
		command = append(command, "--voice")
	}

	if captcha != "" {
		command = append(command, []string{"--captcha", captcha}...)
	}

	_, err := runSignalCli(true, command, "", s.signalCliMode)
	return err
}

func (s *SignalClient) VerifyRegisteredNumber(number string, token string, pin string) error {
	cmd := []string{"--config", s.signalCliConfig, "-u", number, "verify", token}
	if pin != "" {
		cmd = append(cmd, "--pin")
		cmd = append(cmd, pin)
	}

	_, err := runSignalCli(true, cmd, "", s.signalCliMode)
	return err
}

func (s *SignalClient) SendV1(number string, message string, recipients []string, base64Attachments []string, isGroup bool) (*SendResponse, error) {
	timestamp, err := s.send(number, message, recipients, base64Attachments, isGroup)
	return timestamp, err
}

func (s *SignalClient) getJsonRpc2Client(number string) (*JsonRpc2Client, error) {
	if val, ok := s.jsonRpc2Clients[number]; ok {
		return val, nil
	}
	return nil, errors.New("Number not registered with JSON-RPC")
}

func (s *SignalClient) SendV2(number string, message string, recps []string, base64Attachments []string) (*[]SendResponse, error) {
	if len(recps) == 0 {
		return nil, errors.New("Please provide at least one recipient")
	}

	if number == "" {
		return nil, errors.New("Please provide a valid number")
	}

	groups := []string{}
	recipients := []string{}

	for _, recipient := range recps {
		if strings.HasPrefix(recipient, groupPrefix) {
			groups = append(groups, strings.TrimPrefix(recipient, groupPrefix))
		} else {
			recipients = append(recipients, recipient)
		}
	}

	if len(recipients) > 0 && len(groups) > 0 {
		return nil, errors.New("Signal Messenger Groups and phone numbers cannot be specified together in one request! Please split them up into multiple REST API calls.")
	}

	if len(groups) > 1 {
		return nil, errors.New("A signal message cannot be sent to more than one group at once! Please use multiple REST API calls for that.")
	}

	timestamps := []SendResponse{}
	for _, group := range groups {
		timestamp, err := s.send(number, message, []string{group}, base64Attachments, true)
		if err != nil {
			return nil, err
		}
		timestamps = append(timestamps, *timestamp)
	}

	if len(recipients) > 0 {
		timestamp, err := s.send(number, message, recipients, base64Attachments, false)
		if err != nil {
			return nil, err
		}
		timestamps = append(timestamps, *timestamp)
	}

	return &timestamps, nil
}

func (s *SignalClient) Receive(number string, timeout int64) (string, error) {
	command := []string{"--config", s.signalCliConfig, "--output", "json", "-u", number, "receive", "-t", strconv.FormatInt(timeout, 10)}

	out, err := runSignalCli(true, command, "", s.signalCliMode)
	if err != nil {
		return "", err
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

	return jsonStr, nil
}

func (s *SignalClient) CreateGroup(number string, name string, members []string, description string, editGroupPermission GroupPermission, addMembersPermission GroupPermission, groupLinkState GroupLinkState) (string, error) {
	var err error
	var rawData string
	if s.signalCliMode == JsonRpc {
		type Request struct {
			Name    string   `json:"name"`
			Members []string `json:"members"`
		}
		request := Request{Name: name, Members: members}

		jsonRpc2Client, err := s.getJsonRpc2Client(number)
		if err != nil {
			return "", err
		}
		rawData, err = jsonRpc2Client.getRaw("updateGroup", request)
		if err != nil {
			return "", err
		}
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-u", number, "updateGroup", "-n", name, "-m"}
		cmd = append(cmd, members...)

		if addMembersPermission != DefaultGroupPermission {
			cmd = append(cmd, []string{"--set-permission-add-member", addMembersPermission.String()}...)
		}

		if editGroupPermission != DefaultGroupPermission {
			cmd = append(cmd, []string{"--set-permission-edit-details", editGroupPermission.String()}...)
		}

		if groupLinkState != DefaultGroupLinkState {
			cmd = append(cmd, []string{"--link", groupLinkState.String()}...)
		}

		if description != "" {
			cmd = append(cmd, []string{"--description", description}...)
		}

		rawData, err = runSignalCli(true, cmd, "", s.signalCliMode)
		if err != nil {
			if strings.Contains(err.Error(), signalCliV2GroupError) {
				return "", errors.New("Cannot create group - please first update your profile.")
			}
			return "", err
		}
	}
	internalGroupId := getStringInBetween(rawData, `"`, `"`)
	groupId := convertInternalGroupIdToGroupId(internalGroupId)

	return groupId, nil
}

func (s *SignalClient) GetGroups(number string) ([]GroupEntry, error) {
	groupEntries := []GroupEntry{}

	var signalCliGroupEntries []SignalCliGroupEntry
	var err error
	var rawData string

	if s.signalCliMode == JsonRpc {
		jsonRpc2Client, err := s.getJsonRpc2Client(number)
		if err != nil {
			return groupEntries, err
		}
		rawData, err = jsonRpc2Client.getRaw("listGroups", nil)
		if err != nil {
			return groupEntries, err
		}
	} else {
		rawData, err = runSignalCli(true, []string{"--config", s.signalCliConfig, "--output", "json", "-u", number, "listGroups", "-d"}, "", s.signalCliMode)
		if err != nil {
			return groupEntries, err
		}
	}

	err = json.Unmarshal([]byte(rawData), &signalCliGroupEntries)
	if err != nil {
		return groupEntries, err
	}

	for _, signalCliGroupEntry := range signalCliGroupEntries {
		var groupEntry GroupEntry
		groupEntry.InternalId = signalCliGroupEntry.Id
		groupEntry.Name = signalCliGroupEntry.Name
		groupEntry.Id = convertInternalGroupIdToGroupId(signalCliGroupEntry.Id)
		groupEntry.Blocked = signalCliGroupEntry.IsBlocked

		members := []string{}
		for _, val := range signalCliGroupEntry.Members {
			members = append(members, val.Number)
		}
		groupEntry.Members = members

		groupEntry.PendingRequests = signalCliGroupEntry.PendingMembers
		groupEntry.PendingInvites = signalCliGroupEntry.RequestingMembers
		groupEntry.InviteLink = signalCliGroupEntry.GroupInviteLink

		groupEntries = append(groupEntries, groupEntry)
	}

	return groupEntries, nil
}

func (s *SignalClient) GetGroup(number string, groupId string) (*GroupEntry, error) {
	groupEntry := GroupEntry{}
	groups, err := s.GetGroups(number)
	if err != nil {
		return nil, err
	}

	for _, group := range groups {
		if group.Id == groupId {
			groupEntry = group
			return &groupEntry, nil
		}
	}

	return nil, nil
}

func (s *SignalClient) DeleteGroup(number string, groupId string) error {
	_, err := runSignalCli(true, []string{"--config", s.signalCliConfig, "-u", number, "quitGroup", "-g", string(groupId)}, "", s.signalCliMode)
	return err
}

func (s *SignalClient) GetQrCodeLink(deviceName string) ([]byte, error) {
	command := []string{"--config", s.signalCliConfig, "link", "-n", deviceName}

	tsdeviceLink, err := runSignalCli(false, command, "", s.signalCliMode)
	if err != nil {
		return []byte{}, errors.New("Couldn't create QR code: " + err.Error())
	}

	q, err := qrcode.New(string(tsdeviceLink), qrcode.Medium)
	if err != nil {
		return []byte{}, errors.New("Couldn't create QR code: " + err.Error())
	}

	q.DisableBorder = false
	var png []byte
	png, err = q.PNG(256)
	if err != nil {
		return []byte{}, errors.New("Couldn't create QR code: " + err.Error())
	}
	return png, nil
}

func (s *SignalClient) GetAttachments() ([]string, error) {
	files := []string{}

	err := filepath.Walk(s.signalCliConfig+"/attachments/", func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		files = append(files, filepath.Base(path))
		return nil
	})

	return files, err
}

func (s *SignalClient) RemoveAttachment(attachment string) error {
	path, err := securejoin.SecureJoin(s.signalCliConfig+"/attachments/", attachment)
	if err != nil {
		return &InvalidNameError{Description: "Please provide a valid attachment name"}
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &NotFoundError{Description: "No attachment with that name found"}
	}
	err = os.Remove(path)
	if err != nil {
		return &InternalError{Description: "Couldn't delete attachment - please try again later"}
	}

	return nil
}

func (s *SignalClient) GetAttachment(attachment string) ([]byte, error) {
	path, err := securejoin.SecureJoin(s.signalCliConfig+"/attachments/", attachment)
	if err != nil {
		return []byte{}, &InvalidNameError{Description: "Please provide a valid attachment name"}
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []byte{}, &NotFoundError{Description: "No attachment with that name found"}
	}

	attachmentBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return []byte{}, &InternalError{Description: "Couldn't read attachment - please try again later"}
	}

	return attachmentBytes, nil
}

func (s *SignalClient) UpdateProfile(number string, profileName string, base64Avatar string) error {
	var err error
	var avatarTmpPath string
	if base64Avatar != "" {
		u, err := uuid.NewV4()
		if err != nil {
			return err
		}

		avatarBytes, err := base64.StdEncoding.DecodeString(base64Avatar)
		if err != nil {
			return errors.New("Couldn't decode base64 encoded avatar: " + err.Error())
		}

		fType, err := filetype.Get(avatarBytes)
		if err != nil {
			return err
		}

		avatarTmpPath := s.avatarTmpDir + u.String() + "." + fType.Extension

		f, err := os.Create(avatarTmpPath)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := f.Write(avatarBytes); err != nil {
			cleanupTmpFiles([]string{avatarTmpPath})
			return err
		}
		if err := f.Sync(); err != nil {
			cleanupTmpFiles([]string{avatarTmpPath})
			return err
		}
		f.Close()
	}

	if s.signalCliMode == JsonRpc {
		type Request struct {
			Name         string `json:"name"`
			Avatar       string `json:"avatar,omitempty"`
			RemoveAvatar bool   `json:"remove-avatar"`
		}
		request := Request{Name: profileName}
		if base64Avatar == "" {
			request.RemoveAvatar = true
		} else {
			request.Avatar = avatarTmpPath
			request.RemoveAvatar = false
		}
		jsonRpc2Client, err := s.getJsonRpc2Client(number)
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("updateProfile", request)
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-u", number, "updateProfile", "--name", profileName}
		if base64Avatar == "" {
			cmd = append(cmd, "--remove-avatar")
		} else {
			cmd = append(cmd, []string{"--avatar", avatarTmpPath}...)
		}

		_, err = runSignalCli(true, cmd, "", s.signalCliMode)
	}

	cleanupTmpFiles([]string{avatarTmpPath})
	return err
}

func (s *SignalClient) ListIdentities(number string) (*[]IdentityEntry, error) {
	identityEntries := []IdentityEntry{}
	if s.signalCliMode == JsonRpc {
		jsonRpc2Client, err := s.getJsonRpc2Client(number)
		if err != nil {
			return nil, err
		}
		rawData, err := jsonRpc2Client.getRaw("listIdentities", nil)
		signalCliIdentityEntries := []SignalCliIdentityEntry{}
		err = json.Unmarshal([]byte(rawData), &signalCliIdentityEntries)
		if err != nil {
			return nil, err
		}
		for _, signalCliIdentityEntry := range signalCliIdentityEntries {
			identityEntry := IdentityEntry{
				Number:       signalCliIdentityEntry.Number,
				Status:       signalCliIdentityEntry.TrustLevel,
				Added:        strconv.FormatInt(signalCliIdentityEntry.AddedTimestamp, 10),
				Fingerprint:  signalCliIdentityEntry.Fingerprint,
				SafetyNumber: signalCliIdentityEntry.SafetyNumber,
			}
			identityEntries = append(identityEntries, identityEntry)
		}
	} else {
		rawData, err := runSignalCli(true, []string{"--config", s.signalCliConfig, "-u", number, "listIdentities"}, "", s.signalCliMode)
		if err != nil {
			return nil, err
		}

		keyValuePairs := parseWhitespaceDelimitedKeyValueStringList(rawData, []string{"NumberAndTrustStatus", "Added", "Fingerprint", "Safety Number"})
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
	}

	return &identityEntries, nil
}

func (s *SignalClient) TrustIdentity(number string, numberToTrust string, verifiedSafetyNumber string) error {
	var err error
	if s.signalCliMode == JsonRpc {
		type Request struct {
			VerifiedSafetyNumber string `json:"verified-safety-number"`
			Recipient            string `json:"recipient"`
		}
		request := Request{VerifiedSafetyNumber: verifiedSafetyNumber, Recipient: numberToTrust}
		jsonRpc2Client, err := s.getJsonRpc2Client(number)
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("trust", request)
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-u", number, "trust", numberToTrust, "--verified-safety-number", verifiedSafetyNumber}
		_, err = runSignalCli(true, cmd, "", s.signalCliMode)
	}
	return err
}

func (s *SignalClient) BlockGroup(number string, groupId string) error {
	_, err := runSignalCli(true, []string{"--config", s.signalCliConfig, "-u", number, "block", "-g", groupId}, "", s.signalCliMode)
	return err
}

func (s *SignalClient) JoinGroup(number string, groupId string) error {
	var err error
	if s.signalCliMode == JsonRpc {
		type Request struct {
			GroupId string `json:"groupId"`
		}
		request := Request{GroupId: groupId}
		jsonRpc2Client, err := s.getJsonRpc2Client(number)
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("updateGroup", request)
	} else {
		_, err = runSignalCli(true, []string{"--config", s.signalCliConfig, "-u", number, "updateGroup", "-g", groupId}, "", s.signalCliMode)
	}
	return err
}

func (s *SignalClient) QuitGroup(number string, groupId string) error {
	var err error
	if s.signalCliMode == JsonRpc {
		type Request struct {
			GroupId string `json:"groupId"`
		}
		request := Request{GroupId: groupId}
		jsonRpc2Client, err := s.getJsonRpc2Client(number)
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("quitGroup", request)
	} else {
		_, err = runSignalCli(true, []string{"--config", s.signalCliConfig, "-u", number, "quitGroup", "-g", groupId}, "", s.signalCliMode)
	}
	return err
}
