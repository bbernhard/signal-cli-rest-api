package client

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/h2non/filetype"

	uuid "github.com/gofrs/uuid"
	qrcode "github.com/skip2/go-qrcode"

	ds "github.com/bbernhard/signal-cli-rest-api/datastructs"
	utils "github.com/bbernhard/signal-cli-rest-api/utils"
)

const groupPrefix = "group."

const signalCliV2GroupError = "Cannot create a V2 group as self does not have a versioned profile"

const endpointNotSupportedInJsonRpcMode = "This endpoint is not supported in JSON-RPC mode."

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
	switch g {
	case DefaultGroupPermission:
		return ""
	case EveryMember:
		return "every-member"
	case OnlyAdmins:
		return "only-admins"
	}
	return ""
}

func (g GroupPermission) FromString(input string) GroupPermission {
	if input == "every-member" {
		return EveryMember
	}
	if input == "only-admins" {
		return OnlyAdmins
	}
	return DefaultGroupPermission
}

func (g GroupLinkState) String() string {
	switch g {
	case DefaultGroupLinkState:
		return ""
	case Enabled:
		return "enabled"
	case EnabledWithApproval:
		return "enabled-with-approval"
	case Disabled:
		return "disabled"
	}
	return ""
}

func (g GroupLinkState) FromString(input string) GroupLinkState {
	if input == "enabled" {
		return Enabled
	}
	if input == "enabled-with-approval" {
		return EnabledWithApproval
	}
	if input == "disabled" {
		return Disabled
	}

	return DefaultGroupLinkState
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
	Admins          []string `json:"admins"`
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

type SignalCliGroupAdmin struct {
	Number string `json:"number"`
	Uuid   string `json:"uuid"`
}

type SignalCliGroupEntry struct {
	Name              string                 `json:"name"`
	Id                string                 `json:"id"`
	IsMember          bool                   `json:"isMember"`
	IsBlocked         bool                   `json:"isBlocked"`
	Members           []SignalCliGroupMember `json:"members"`
	PendingMembers    []SignalCliGroupMember `json:"pendingMembers"`
	RequestingMembers []SignalCliGroupMember `json:"requestingMembers"`
	GroupInviteLink   string                 `json:"groupInviteLink"`
	Admins            []SignalCliGroupAdmin  `json:"admins"`
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
	SupportedApiVersions []string            `json:"versions"`
	BuildNr              int                 `json:"build"`
	Mode                 string              `json:"mode"`
	Version              string              `json:"version"`
	Capabilities         map[string][]string `json:"capabilities"`
}

type SearchResultEntry struct {
	Number     string `json:"number"`
	Registered bool   `json:"registered"`
}

type SetUsernameResponse struct {
	Username     string `json:"username"`
	UsernameLink string `json:"username_link"`
}

type ListInstalledStickerPacksResponse struct {
	PackId    string `json:"pack_id"`
	Url       string `json:"url"`
	Installed bool   `json:"installed"`
	Title     string `json:"title"`
	Author    string `json:"author"`
}

type ListContactsResponse struct {
	Number            string `json:"number"`
	Uuid              string `json:"uuid"`
	Name              string `json:"name"`
	ProfileName       string `json:"profile_name"`
	Username          string `json:"username"`
	Color             string `json:"color"`
	Blocked           bool   `json:"blocked"`
	MessageExpiration string `json:"message_expiration"`
}

func cleanupTmpFiles(paths []string) {
	for _, path := range paths {
		os.Remove(path)
	}
}

func cleanupAttachmentEntries(attachmentEntries []AttachmentEntry) {
	for _, attachmentEntry := range attachmentEntries {
		attachmentEntry.cleanUp()
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

func ConvertGroupIdToInternalGroupId(id string) (string, error) {

	groupIdWithoutPrefix := strings.TrimPrefix(id, groupPrefix)
	internalGroupId, err := base64.StdEncoding.DecodeString(groupIdWithoutPrefix)
	if err != nil {
		return "", errors.New("Invalid group id")
	}

	return string(internalGroupId), err
}

func getSignalCliModeString(signalCliMode SignalCliMode) string {
	if signalCliMode == Normal {
		return "normal"
	} else if signalCliMode == Native {
		return "native"
	} else if signalCliMode == JsonRpc {
		return "json-rpc"
	}
	return "unknown"
}

func getRecipientType(s string) (ds.RecpType, error) {
	// check if the provided recipient is of type 'group'
	if strings.HasPrefix(s, groupPrefix) { // if the recipient starts with 'group.' it is either a group or a username that starts with 'group.'
		// in order to find out whether it is a Signal group or a username that starts with 'group.',
		// we remove the prefix and attempt to base64 decode the group name twice (in case it is a Signal group, the group name was base64 encoded
		// twice - once in the REST API wrapper and once in signal-cli). If the decoded Signal Group is 32 in length, we know that it is a Signal Group.
		// A Signal Group is exactly 32 elements long (see https://github.com/signalapp/libsignal/blob/1086531d798fb4bde25dfaba51ecb59500e0715f/rust/zkgroup/src/api/groups/group_params.rs#L69), whereas the Signal Username Discriminator can be at most 10 digits long (see https://signal.miraheze.org/wiki/Usernames#Discriminator).
		// So in case the group name is 32 elements long we know for sure that it is a Signal Group.
		s1 := strings.TrimPrefix(s, groupPrefix)
		signalCliBase64EncodedGroupId, err := base64.StdEncoding.DecodeString(s1)
		if err == nil {
			signalCliGroupId, err := base64.StdEncoding.DecodeString(string(signalCliBase64EncodedGroupId))
			if err == nil {
				if len(signalCliGroupId) == 32 {
					return ds.Group, nil
				} else {
					return ds.Group, errors.New("Invalid Signal group size (" + strconv.Itoa(len(signalCliGroupId)))
				}
			}
		} else if len(s1) <= 10 {
			return ds.Username, nil
		}
		return ds.Group, errors.New("Invalid identifier " + s)
	} else if utils.IsPhoneNumber(s) {
		return ds.Number, nil
	} else {
		//last but not least, check if it is a valid uuid.
		//(although it is not directly exposed in the signal-cli manpage, signal-cli allows
		//to send messages to the 'sourceUuid' (which is a UUID)
		_, err := uuid.FromString(s)
		if err == nil {
			return ds.Number, nil
		}
	}
	return ds.Username, nil
}

type SignalClient struct {
	signalCliConfig          string
	attachmentTmpDir         string
	avatarTmpDir             string
	signalCliMode            SignalCliMode
	jsonRpc2ClientConfig     *utils.JsonRpc2ClientConfig
	jsonRpc2ClientConfigPath string
	jsonRpc2Clients          map[string]*JsonRpc2Client
	signalCliApiConfigPath   string
	signalCliApiConfig       *utils.SignalCliApiConfig
	cliClient                *CliClient
}

func NewSignalClient(signalCliConfig string, attachmentTmpDir string, avatarTmpDir string, signalCliMode SignalCliMode,
	jsonRpc2ClientConfigPath string, signalCliApiConfigPath string) *SignalClient {
	return &SignalClient{
		signalCliConfig:          signalCliConfig,
		attachmentTmpDir:         attachmentTmpDir,
		avatarTmpDir:             avatarTmpDir,
		signalCliMode:            signalCliMode,
		jsonRpc2ClientConfigPath: jsonRpc2ClientConfigPath,
		jsonRpc2Clients:          make(map[string]*JsonRpc2Client),
		signalCliApiConfigPath:   signalCliApiConfigPath,
	}
}

func (s *SignalClient) GetSignalCliMode() SignalCliMode {
	return s.signalCliMode
}

func (s *SignalClient) Init() error {
	s.signalCliApiConfig = utils.NewSignalCliApiConfig()
	err := s.signalCliApiConfig.Load(s.signalCliApiConfigPath)
	if err != nil {
		return err
	}

	if s.signalCliMode == JsonRpc {
		s.jsonRpc2ClientConfig = utils.NewJsonRpc2ClientConfig()
		err := s.jsonRpc2ClientConfig.Load(s.jsonRpc2ClientConfigPath)
		if err != nil {
			return err
		}

		tcpPortsNumberMapping := s.jsonRpc2ClientConfig.GetTcpPortsForNumbers()
		for number, tcpPort := range tcpPortsNumberMapping {
			s.jsonRpc2Clients[number] = NewJsonRpc2Client(s.signalCliApiConfig, number)
			err := s.jsonRpc2Clients[number].Dial("127.0.0.1:" + strconv.FormatInt(tcpPort, 10))
			if err != nil {
				return err
			}

			go s.jsonRpc2Clients[number].ReceiveData(number) //receive messages in goroutine
		}
	} else {
		s.cliClient = NewCliClient(s.signalCliMode, s.signalCliApiConfig)
	}

	return nil
}

func (s *SignalClient) send(signalCliSendRequest ds.SignalCliSendRequest) (*SendResponse, error) {
	var resp SendResponse

	if len(signalCliSendRequest.Recipients) == 0 {
		return nil, errors.New("Please specify at least one recipient")
	}

	signalCliTextFormatStrings := []string{}
	if signalCliSendRequest.TextMode != nil && *signalCliSendRequest.TextMode == "styled" {
		signalCliSendRequest.Message, signalCliTextFormatStrings = utils.ParseMarkdownMessage(signalCliSendRequest.Message)
	}

	var groupId string = ""
	if signalCliSendRequest.RecipientType == ds.Group {
		if len(signalCliSendRequest.Recipients) > 1 {
			return nil, errors.New("More than one recipient is currently not allowed")
		}

		grpId, err := base64.StdEncoding.DecodeString(signalCliSendRequest.Recipients[0])
		if err != nil {
			return nil, errors.New("Invalid group id")
		}
		groupId = string(grpId)
	}

	attachmentEntries := []AttachmentEntry{}
	for _, base64Attachment := range signalCliSendRequest.Base64Attachments {
		attachmentEntry := NewAttachmentEntry(base64Attachment, s.attachmentTmpDir)

		err := attachmentEntry.storeBase64AsTemporaryFile()
		if err != nil {
			cleanupAttachmentEntries(attachmentEntries)
			return nil, err
		}

		attachmentEntries = append(attachmentEntries, *attachmentEntry)
	}

	if s.signalCliMode == JsonRpc {
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return nil, err
		}

		type Request struct {
			Recipients     []string `json:"recipient,omitempty"`
			Usernames      []string `json:"username,omitempty"`
			Message        string   `json:"message"`
			GroupId        string   `json:"group-id,omitempty"`
			Attachments    []string `json:"attachment,omitempty"`
			Sticker        string   `json:"sticker,omitempty"`
			Mentions       []string `json:"mentions,omitempty"`
			QuoteTimestamp *int64   `json:"quote-timestamp,omitempty"`
			QuoteAuthor    *string  `json:"quote-author,omitempty"`
			QuoteMessage   *string  `json:"quote-message,omitempty"`
			QuoteMentions  []string `json:"quote-mentions,omitempty"`
			TextStyles     []string `json:"text-style,omitempty"`
			EditTimestamp  *int64   `json:"edit-timestamp,omitempty"`
			NotifySelf     bool     `json:"notify-self,omitempty"`
		}

		request := Request{Message: signalCliSendRequest.Message}
		if signalCliSendRequest.RecipientType == ds.Group {
			request.GroupId = groupId
		} else if signalCliSendRequest.RecipientType == ds.Number {
			request.Recipients = signalCliSendRequest.Recipients
		} else if signalCliSendRequest.RecipientType == ds.Username {
			request.Usernames = signalCliSendRequest.Recipients
		}
		for _, attachmentEntry := range attachmentEntries {
			request.Attachments = append(request.Attachments, attachmentEntry.toDataForSignal())
		}

		// for backwards compatibility, if flag is not set we'll assume that self notification is desired
		if signalCliSendRequest.NotifySelf == nil || *signalCliSendRequest.NotifySelf {
			request.NotifySelf = true
		}

		request.Sticker = signalCliSendRequest.Sticker
		if signalCliSendRequest.Mentions != nil {
			request.Mentions = make([]string, len(signalCliSendRequest.Mentions))
			for i, mention := range signalCliSendRequest.Mentions {
				request.Mentions[i] = mention.ToString()
			}
		} else {
			request.Mentions = nil
		}
		request.QuoteTimestamp = signalCliSendRequest.QuoteTimestamp
		request.QuoteAuthor = signalCliSendRequest.QuoteAuthor
		request.QuoteMessage = signalCliSendRequest.QuoteMessage
		if signalCliSendRequest.QuoteMentions != nil {
			request.QuoteMentions = make([]string, len(signalCliSendRequest.QuoteMentions))
			for i, mention := range signalCliSendRequest.QuoteMentions {
				request.QuoteMentions[i] = mention.ToString()
			}
		} else {
			request.QuoteMentions = nil
		}
		request.EditTimestamp = signalCliSendRequest.EditTimestamp

		if len(signalCliTextFormatStrings) > 0 {
			request.TextStyles = signalCliTextFormatStrings
		}

		rawData, err := jsonRpc2Client.getRaw("send", &signalCliSendRequest.Number, request)
		if err != nil {
			cleanupAttachmentEntries(attachmentEntries)
			return nil, err
		}

		err = json.Unmarshal([]byte(rawData), &resp)
		if err != nil {
			cleanupAttachmentEntries(attachmentEntries)

			if strings.Contains(err.Error(), signalCliV2GroupError) {
				return nil, errors.New("Cannot send message to group - please first update your profile.")
			}
			return nil, err
		}
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", signalCliSendRequest.Number, "send", "--message-from-stdin"}
		if signalCliSendRequest.RecipientType == ds.Number {
			cmd = append(cmd, signalCliSendRequest.Recipients...)
		} else if signalCliSendRequest.RecipientType == ds.Group {
			cmd = append(cmd, []string{"-g", groupId}...)
		} else if signalCliSendRequest.RecipientType == ds.Username {
			cmd = append(cmd, "-u")
			cmd = append(cmd, signalCliSendRequest.Recipients...)
		}

		if len(signalCliTextFormatStrings) > 0 {
			cmd = append(cmd, "--text-style")
			cmd = append(cmd, signalCliTextFormatStrings...)
		}

		if len(attachmentEntries) > 0 {
			cmd = append(cmd, "-a")
			for _, attachmentEntry := range attachmentEntries {
				cmd = append(cmd, attachmentEntry.toDataForSignal())
			}
		}

		for _, mention := range signalCliSendRequest.Mentions {
			cmd = append(cmd, "--mention")
			cmd = append(cmd, mention.ToString())
		}

		if signalCliSendRequest.Sticker != "" {
			cmd = append(cmd, "--sticker")
			cmd = append(cmd, signalCliSendRequest.Sticker)
		}

		if signalCliSendRequest.QuoteTimestamp != nil {
			cmd = append(cmd, "--quote-timestamp")
			cmd = append(cmd, strconv.FormatInt(*signalCliSendRequest.QuoteTimestamp, 10))
		}

		if signalCliSendRequest.QuoteAuthor != nil {
			cmd = append(cmd, "--quote-author")
			cmd = append(cmd, *signalCliSendRequest.QuoteAuthor)
		}

		if signalCliSendRequest.QuoteMessage != nil {
			cmd = append(cmd, "--quote-message")
			cmd = append(cmd, *signalCliSendRequest.QuoteMessage)
		}

		for _, mention := range signalCliSendRequest.QuoteMentions {
			cmd = append(cmd, "--quote-mention")
			cmd = append(cmd, mention.ToString())
		}

		if signalCliSendRequest.EditTimestamp != nil {
			cmd = append(cmd, "--edit-timestamp")
			cmd = append(cmd, strconv.FormatInt(*signalCliSendRequest.EditTimestamp, 10))
		}

		// for backwards compatibility, if nothing is set, use the notify-self flag
		if signalCliSendRequest.NotifySelf == nil || *signalCliSendRequest.NotifySelf {
			cmd = append(cmd, "--notify-self")
		}

		rawData, err := s.cliClient.Execute(true, cmd, signalCliSendRequest.Message)
		if err != nil {
			cleanupAttachmentEntries(attachmentEntries)
			if strings.Contains(err.Error(), signalCliV2GroupError) {
				return nil, errors.New("Cannot send message to group - please first update your profile.")
			}
			return nil, err
		}
		resp.Timestamp, err = strconv.ParseInt(strings.TrimSuffix(rawData, "\n"), 10, 64)
		if err != nil {
			cleanupAttachmentEntries(attachmentEntries)
			return nil, errors.New(strings.Replace(rawData, "\n", "", -1)) //in case we can't parse the timestamp, it means signal-cli threw an error. So instead of returning the parsing error, return the actual error from signal-cli
		}
	}

	cleanupAttachmentEntries(attachmentEntries)

	return &resp, nil
}

func (s *SignalClient) About() About {
	about := About{
		SupportedApiVersions: []string{"v1", "v2"},
		BuildNr:              2,
		Mode:                 getSignalCliModeString(s.signalCliMode),
		Version:              utils.GetEnv("BUILD_VERSION", "unset"),
		Capabilities:         map[string][]string{"v2/send": []string{"quotes", "mentions"}},
	}
	return about
}

func (s *SignalClient) RegisterNumber(number string, useVoice bool, captcha string) error {
	if s.signalCliMode == JsonRpc {
		type Request struct {
			UseVoice bool   `json:"voice,omitempty"`
			Captcha  string `json:"captcha,omitempty"`
			Account  string `json:"account,omitempty"`
		}
		request := Request{Account: number}

		if useVoice {
			request.UseVoice = useVoice
		}

		if captcha != "" {
			request.Captcha = captcha
		}

		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("register", nil, request)
		return err
	} else {
		command := []string{"--config", s.signalCliConfig, "-a", number, "register"}

		if useVoice {
			command = append(command, "--voice")
		}

		if captcha != "" {
			command = append(command, []string{"--captcha", captcha}...)
		}

		_, err := s.cliClient.Execute(true, command, "")
		return err
	}
}

func (s *SignalClient) UnregisterNumber(number string, deleteAccount bool, deleteLocalData bool) error {
	if s.signalCliMode == JsonRpc {
		return errors.New("This functionality is only available in normal/native mode!")
	}

	command := []string{"--config", s.signalCliConfig, "-a", number, "unregister"}
	if deleteAccount {
		command = append(command, "--delete-account")
	}

	_, err := s.cliClient.Execute(true, command, "")

	if deleteLocalData {
		command := []string{"--config", s.signalCliConfig, "-a", number, "deleteLocalAccountData"}
		_, err2 := s.cliClient.Execute(true, command, "")
		if (err2 != nil) && (err != nil) {
			err = fmt.Errorf("%w (%s)", err, err2.Error())
		} else if (err2 != nil) && (err == nil) {
			err = err2
		}
	}

	return err
}

func (s *SignalClient) VerifyRegisteredNumber(number string, token string, pin string) error {
	if s.signalCliMode == JsonRpc {
		type Request struct {
			VerificationCode string `json:"verificationCode,omitempty"`
			Account          string `json:"account,omitempty"`
			Pin              string `json:"pin,omitempty"`
		}
		request := Request{Account: number, VerificationCode: token}

		if pin != "" {
			request.Pin = pin
		}

		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("verify", nil, request)
		return err
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", number, "verify", token}
		if pin != "" {
			cmd = append(cmd, "--pin")
			cmd = append(cmd, pin)
		}

		_, err := s.cliClient.Execute(true, cmd, "")
		return err
	}
}

func (s *SignalClient) SendV1(number string, message string, recipients []string, base64Attachments []string, isGroup bool) (*SendResponse, error) {
	recipientType := ds.Number
	if isGroup {
		recipientType = ds.Group
	}

	signalCliSendRequest := ds.SignalCliSendRequest{Number: number, Message: message, Recipients: recipients, Base64Attachments: base64Attachments,
		RecipientType: recipientType, Sticker: "", Mentions: nil, QuoteTimestamp: nil, QuoteAuthor: nil, QuoteMessage: nil,
		QuoteMentions: nil, TextMode: nil, EditTimestamp: nil}
	timestamp, err := s.send(signalCliSendRequest)
	return timestamp, err
}

func (s *SignalClient) getJsonRpc2Client() (*JsonRpc2Client, error) {
	if val, ok := s.jsonRpc2Clients[utils.MULTI_ACCOUNT_NUMBER]; ok {
		return val, nil
	}
	return nil, errors.New("Number not registered with JSON-RPC")
}

func (s *SignalClient) getJsonRpc2Clients() []*JsonRpc2Client {
	jsonRpc2Clients := []*JsonRpc2Client{}
	for _, client := range s.jsonRpc2Clients {
		jsonRpc2Clients = append(jsonRpc2Clients, client)
	}
	return jsonRpc2Clients
}

func (s *SignalClient) SendV2(number string, message string, recps []string, base64Attachments []string, sticker string, mentions []ds.MessageMention,
	quoteTimestamp *int64, quoteAuthor *string, quoteMessage *string, quoteMentions []ds.MessageMention, textMode *string, editTimestamp *int64, notifySelf *bool) (*[]SendResponse, error) {
	if len(recps) == 0 {
		return nil, errors.New("Please provide at least one recipient")
	}

	if number == "" {
		return nil, errors.New("Please provide a valid number")
	}

	groups := []string{}
	numbers := []string{}
	usernames := []string{}

	for _, recipient := range recps {
		recipientType, err := getRecipientType(recipient)
		if err != nil {
			return nil, err
		}

		if recipientType == ds.Group {
			groups = append(groups, strings.TrimPrefix(recipient, groupPrefix))
		} else if recipientType == ds.Number {
			numbers = append(numbers, recipient)
		} else if recipientType == ds.Username {
			usernames = append(usernames, recipient)
		} else {
			return nil, errors.New("Invalid recipient type")
		}
	}

	if len(numbers) > 0 && len(groups) > 0 {
		return nil, errors.New("Signal Messenger Groups and phone numbers cannot be specified together in one request! Please split them up into multiple REST API calls.")
	}

	if len(usernames) > 0 && len(groups) > 0 {
		return nil, errors.New("Signal Messenger Groups and usernames cannot be specified together in one request! Please split them up into multiple REST API calls.")
	}

	if len(numbers) > 0 && len(usernames) > 0 {
		return nil, errors.New("Signal Messenger phone numbers and usernames cannot be specified together in one request! Please split them up into multiple REST API calls.")
	}

	if len(groups) > 1 {
		return nil, errors.New("A signal message cannot be sent to more than one group at once! Please use multiple REST API calls for that.")
	}

	timestamps := []SendResponse{}
	for _, group := range groups {
		signalCliSendRequest := ds.SignalCliSendRequest{Number: number, Message: message, Recipients: []string{group}, Base64Attachments: base64Attachments,
			RecipientType: ds.Group, Sticker: sticker, Mentions: mentions, QuoteTimestamp: quoteTimestamp,
			QuoteAuthor: quoteAuthor, QuoteMessage: quoteMessage, QuoteMentions: quoteMentions,
			TextMode: textMode, EditTimestamp: editTimestamp, NotifySelf: notifySelf}
		timestamp, err := s.send(signalCliSendRequest)
		if err != nil {
			return nil, err
		}
		timestamps = append(timestamps, *timestamp)
	}

	if len(numbers) > 0 {
		signalCliSendRequest := ds.SignalCliSendRequest{Number: number, Message: message, Recipients: numbers, Base64Attachments: base64Attachments,
			RecipientType: ds.Number, Sticker: sticker, Mentions: mentions, QuoteTimestamp: quoteTimestamp,
			QuoteAuthor: quoteAuthor, QuoteMessage: quoteMessage, QuoteMentions: quoteMentions,
			TextMode: textMode, EditTimestamp: editTimestamp}
		timestamp, err := s.send(signalCliSendRequest)
		if err != nil {
			return nil, err
		}
		timestamps = append(timestamps, *timestamp)
	}

	if len(usernames) > 0 {
		signalCliSendRequest := ds.SignalCliSendRequest{Number: number, Message: message, Recipients: usernames, Base64Attachments: base64Attachments,
			RecipientType: ds.Username, Sticker: sticker, Mentions: mentions, QuoteTimestamp: quoteTimestamp,
			QuoteAuthor: quoteAuthor, QuoteMessage: quoteMessage, QuoteMentions: quoteMentions,
			TextMode: textMode, EditTimestamp: editTimestamp}
		timestamp, err := s.send(signalCliSendRequest)
		if err != nil {
			return nil, err
		}
		timestamps = append(timestamps, *timestamp)
	}

	return &timestamps, nil
}

func (s *SignalClient) Receive(number string, timeout int64, ignoreAttachments bool, ignoreStories bool, maxMessages int64, sendReadReceipts bool) (string, error) {
	if s.signalCliMode == JsonRpc {
		return "", errors.New("Not implemented")
	} else {
		command := []string{"--config", s.signalCliConfig, "--output", "json", "-a", number, "receive", "-t", strconv.FormatInt(timeout, 10)}

		if ignoreAttachments {
			command = append(command, "--ignore-attachments")
		}

		if ignoreStories {
			command = append(command, "--ignore-stories")
		}

		if maxMessages > 0 {
			command = append(command, "--max-messages")
			command = append(command, strconv.FormatInt(maxMessages, 10))
		}

		if sendReadReceipts {
			command = append(command, "--send-read-receipts")
		}

		out, err := s.cliClient.Execute(true, command, "")
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
}

func (s *SignalClient) GetReceiveChannel() (chan JsonRpc2ReceivedMessage, string, error) {
	jsonRpc2Client, err := s.getJsonRpc2Client()
	if err != nil {
		return nil, "", err
	}
	return jsonRpc2Client.GetReceiveChannel()
}

func (s *SignalClient) RemoveReceiveChannel(channelUuid string) {
	jsonRpc2Client, err := s.getJsonRpc2Client()
	if err != nil {
		return
	}
	jsonRpc2Client.RemoveReceiveChannel(channelUuid)
}

func (s *SignalClient) CreateGroup(number string, name string, members []string, description string, editGroupPermission GroupPermission, addMembersPermission GroupPermission, groupLinkState GroupLinkState, expirationTime *int) (string, error) {
	var internalGroupId string
	if s.signalCliMode == JsonRpc {
		type Request struct {
			Name                  string   `json:"name"`
			Members               []string `json:"members"`
			Link                  string   `json:"link,omitempty"`
			Description           string   `json:"description,omitempty"`
			EditGroupPermissions  string   `json:"setPermissionEditDetails,omitempty"`
			AddMembersPermissions string   `json:"setPermissionAddMember,omitempty"`
			Expiration            int      `json:"expiration,omitempty"`
		}
		request := Request{Name: name, Members: members}

		if groupLinkState != DefaultGroupLinkState {
			request.Link = groupLinkState.String()
		}

		if description != "" {
			request.Description = description
		}

		if editGroupPermission != DefaultGroupPermission {
			request.EditGroupPermissions = editGroupPermission.String()
		}

		if addMembersPermission != DefaultGroupPermission {
			request.AddMembersPermissions = addMembersPermission.String()
		}

		if expirationTime != nil {
			request.Expiration = *expirationTime
		}

		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return "", err
		}
		rawData, err := jsonRpc2Client.getRaw("updateGroup", &number, request)
		if err != nil {
			return "", err
		}

		type Response struct {
			GroupId   string `json:"groupId"`
			Timestamp int64  `json:"timestamp"`
		}
		var resp Response
		json.Unmarshal([]byte(rawData), &resp)
		if err != nil {
			return "", err
		}
		internalGroupId = resp.GroupId
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", number, "updateGroup", "-n", name, "-m"}
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

		if expirationTime != nil {
			cmd = append(cmd, []string{"--expiration", strconv.Itoa(*expirationTime)}...)
		}

		rawData, err := s.cliClient.Execute(true, cmd, "")
		if err != nil {
			if strings.Contains(err.Error(), signalCliV2GroupError) {
				return "", errors.New("Cannot create group - please first update your profile.")
			}
			return "", err
		}
		internalGroupId = getStringInBetween(rawData, `"`, `"`)
	}
	groupId := convertInternalGroupIdToGroupId(internalGroupId)

	return groupId, nil
}

func (s *SignalClient) updateGroupMembers(number string, groupId string, members []string, add bool) error {
	var err error

	if len(members) == 0 {
		return nil
	}

	group, err := s.GetGroup(number, groupId)
	if err != nil {
		return err
	}

	if group == nil {
		return &NotFoundError{Description: "No group with that group id (" + groupId + ") found"}
	}

	internalGroupId, err := ConvertGroupIdToInternalGroupId(groupId)
	if err != nil {
		return errors.New("Invalid group id")
	}

	if s.signalCliMode == JsonRpc {
		type Request struct {
			Name          string   `json:"name,omitempty"`
			Members       []string `json:"member,omitempty"`
			RemoveMembers []string `json:"remove-member,omitempty"`
			GroupId       string   `json:"groupId"`
		}
		request := Request{GroupId: internalGroupId}
		if add {
			request.Members = append(request.Members, members...)
		} else {
			request.RemoveMembers = append(request.RemoveMembers, members...)
		}

		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("updateGroup", &number, request)
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", number, "updateGroup", "-g", internalGroupId}

		if add {
			cmd = append(cmd, "-m")
		} else {
			cmd = append(cmd, "-r")
		}
		cmd = append(cmd, members...)

		_, err = s.cliClient.Execute(true, cmd, "")
	}
	return err
}

func (s *SignalClient) AddMembersToGroup(number string, groupId string, members []string) error {
	return s.updateGroupMembers(number, groupId, members, true)
}

func (s *SignalClient) RemoveMembersFromGroup(number string, groupId string, members []string) error {
	return s.updateGroupMembers(number, groupId, members, false)
}

func (s *SignalClient) updateGroupAdmins(number string, groupId string, admins []string, add bool) error {
	var err error

	if len(admins) == 0 {
		return nil
	}

	group, err := s.GetGroup(number, groupId)
	if err != nil {
		return err
	}

	if group == nil {
		return &NotFoundError{Description: "No group with that group id (" + groupId + ") found"}
	}

	internalGroupId, err := ConvertGroupIdToInternalGroupId(groupId)
	if err != nil {
		return errors.New("Invalid group id")
	}

	if s.signalCliMode == JsonRpc {
		type Request struct {
			Name         string   `json:"name,omitempty"`
			Admins       []string `json:"admin,omitempty"`
			RemoveAdmins []string `json:"remove-admin,omitempty"`
			GroupId      string   `json:"groupId"`
		}
		request := Request{GroupId: internalGroupId}
		if add {
			request.Admins = append(request.Admins, admins...)
		} else {
			request.RemoveAdmins = append(request.RemoveAdmins, admins...)
		}

		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("updateGroup", &number, request)
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", number, "updateGroup", "-g", internalGroupId}

		if add {
			cmd = append(cmd, "--admin")
		} else {
			cmd = append(cmd, "--remove-admin")
		}
		cmd = append(cmd, admins...)

		_, err = s.cliClient.Execute(true, cmd, "")
	}
	return err
}

func (s *SignalClient) AddAdminsToGroup(number string, groupId string, admins []string) error {
	return s.updateGroupAdmins(number, groupId, admins, true)
}

func (s *SignalClient) RemoveAdminsFromGroup(number string, groupId string, admins []string) error {
	return s.updateGroupAdmins(number, groupId, admins, false)
}

func (s *SignalClient) GetGroups(number string) ([]GroupEntry, error) {
	groupEntries := []GroupEntry{}

	var signalCliGroupEntries []SignalCliGroupEntry
	var err error
	var rawData string

	if s.signalCliMode == JsonRpc {
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return groupEntries, err
		}
		rawData, err = jsonRpc2Client.getRaw("listGroups", &number, nil)
		if err != nil {
			return groupEntries, err
		}
	} else {
		rawData, err = s.cliClient.Execute(true, []string{"--config", s.signalCliConfig, "--output", "json", "-a", number, "listGroups", "-d"}, "")
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

		pendingMembers := []string{}
		for _, val := range signalCliGroupEntry.PendingMembers {
			pendingMembers = append(pendingMembers, val.Number)
		}
		groupEntry.PendingRequests = pendingMembers

		requestingMembers := []string{}
		for _, val := range signalCliGroupEntry.RequestingMembers {
			requestingMembers = append(requestingMembers, val.Number)
		}
		groupEntry.PendingInvites = requestingMembers

		admins := []string{}
		for _, val := range signalCliGroupEntry.Admins {
			admins = append(admins, val.Number)
		}
		groupEntry.Admins = admins

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
	if s.signalCliMode == JsonRpc {
		type Request struct {
			GroupId string `json:"groupId"`
		}
		request := Request{GroupId: groupId}

		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("quitGroup", &number, request)
		return err
	} else {
		ret, err := s.cliClient.Execute(true, []string{"--config", s.signalCliConfig, "-a", number, "quitGroup", "-g", string(groupId)}, "")
		if strings.Contains(ret, "User is not a group member") {
			return errors.New("Can't delete group: User is not a group member")
		}
		return err
	}
}

func (s *SignalClient) GetQrCodeLink(deviceName string, qrCodeVersion int) ([]byte, error) {
	if s.signalCliMode == JsonRpc {
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return []byte{}, err
		}

		type StartRequest struct{}
		type Response struct {
			DeviceLinkUri string `json:"deviceLinkUri"`
		}

		result, err := jsonRpc2Client.getRaw("startLink", nil, &StartRequest{})
		if err != nil {
			return []byte{}, errors.New("Couldn't create QR code: " + err.Error())
		}

		var resp Response
		err = json.Unmarshal([]byte(result), &resp)
		if err != nil {
			return []byte{}, errors.New("Couldn't create QR code: " + err.Error())
		}

		q, err := qrcode.NewWithForcedVersion(string(resp.DeviceLinkUri), qrCodeVersion, qrcode.Highest)
		if err != nil {
			return []byte{}, errors.New("Couldn't create QR code: " + err.Error())
		}

		var png []byte
		png, err = q.PNG(256)
		if err != nil {
			return []byte{}, errors.New("Couldn't create QR code: " + err.Error())
		}

		go (func() {
			type FinishRequest struct {
				DeviceLinkUri string `json:"deviceLinkUri"`
				DeviceName    string `json:"deviceName"`
			}

			req := FinishRequest{
				DeviceLinkUri: resp.DeviceLinkUri,
				DeviceName:    deviceName,
			}

			result, err := jsonRpc2Client.getRaw("finishLink", nil, &req)
			if err != nil {
				log.Debug("Error linking device: ", err.Error())
				return
			}
			log.Debug("Linking device result: ", result)
			s.signalCliApiConfig.Load(s.signalCliApiConfigPath)
		})()

		return png, nil
	}
	command := []string{"--config", s.signalCliConfig, "link", "-n", deviceName}

	tsdeviceLink, err := s.cliClient.Execute(false, command, "")
	if err != nil {
		return []byte{}, errors.New("Couldn't create QR code: " + err.Error())
	}

	q, err := qrcode.NewWithForcedVersion(string(tsdeviceLink), qrCodeVersion, qrcode.Highest)
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

func (s *SignalClient) GetAccounts() ([]string, error) {
	accounts := make([]string, 0)
	var rawData string
	var err error

	if s.signalCliMode == JsonRpc {
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return accounts, err
		}
		rawData, err = jsonRpc2Client.getRaw("listAccounts", nil, nil)
		if err != nil {
			return accounts, err
		}

	} else {
		rawData, err = s.cliClient.Execute(true, []string{"--config", s.signalCliConfig, "--output", "json", "listAccounts"}, "")
		if err != nil {
			return accounts, err
		}
	}

	type Account struct {
		Number string `json:"number"`
	}
	accountObjs := []Account{}

	err = json.Unmarshal([]byte(rawData), &accountObjs)
	if err != nil {
		return accounts, err
	}

	for _, account := range accountObjs {
		accounts = append(accounts, account.Number)
	}

	return accounts, nil
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

func (s *SignalClient) UpdateProfile(number string, profileName string, base64Avatar string, about *string) error {
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

		avatarTmpPath = s.avatarTmpDir + u.String() + "." + fType.Extension

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
			Name         string  `json:"given-name"`
			Avatar       string  `json:"avatar,omitempty"`
			RemoveAvatar bool    `json:"remove-avatar"`
			About        *string `json:"about,omitempty"`
		}
		request := Request{Name: profileName}
		request.About = about
		if base64Avatar == "" {
			request.RemoveAvatar = true
		} else {
			request.Avatar = avatarTmpPath
			request.RemoveAvatar = false
		}

		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("updateProfile", &number, request)
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", number, "updateProfile", "--given-name", profileName}
		if base64Avatar == "" {
			cmd = append(cmd, "--remove-avatar")
		} else {
			cmd = append(cmd, []string{"--avatar", avatarTmpPath}...)
		}

		if about != nil {
			cmd = append(cmd, []string{"--about", *about}...)
		}

		_, err = s.cliClient.Execute(true, cmd, "")
	}

	cleanupTmpFiles([]string{avatarTmpPath})
	return err
}

func (s *SignalClient) ListIdentities(number string) (*[]IdentityEntry, error) {
	identityEntries := []IdentityEntry{}
	if s.signalCliMode == JsonRpc {
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return nil, err
		}
		rawData, err := jsonRpc2Client.getRaw("listIdentities", &number, nil)
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
		rawData, err := s.cliClient.Execute(true, []string{"--config", s.signalCliConfig, "-a", number, "listIdentities"}, "")
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

func (s *SignalClient) TrustIdentity(number string, numberToTrust string, verifiedSafetyNumber *string, trustAllKnownKeys *bool) error {
	var err error
	if s.signalCliMode == JsonRpc {
		type Request struct {
			VerifiedSafetyNumber string `json:"verified-safety-number,omitempty"`
			TrustAllKnownKeys    bool   `json:"trust-all-known-keys,omitempty"`
			Recipient            string `json:"recipient"`
		}
		request := Request{Recipient: numberToTrust}

		if verifiedSafetyNumber != nil {
			request.VerifiedSafetyNumber = *verifiedSafetyNumber
		}

		if trustAllKnownKeys != nil {
			request.TrustAllKnownKeys = *trustAllKnownKeys
		}

		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("trust", &number, request)
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", number, "trust", numberToTrust}

		if verifiedSafetyNumber != nil {
			cmd = append(cmd, []string{"--verified-safety-number", *verifiedSafetyNumber}...)
		}

		if trustAllKnownKeys != nil && *trustAllKnownKeys {
			cmd = append(cmd, "--trust-all-known-keys")
		}

		_, err = s.cliClient.Execute(true, cmd, "")
	}
	return err
}

func (s *SignalClient) BlockGroup(number string, groupId string) error {
	var err error
	if s.signalCliMode == JsonRpc {
		type Request struct {
			GroupId string `json:"groupId"`
		}
		request := Request{GroupId: groupId}
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("block", &number, request)
	} else {
		_, err = s.cliClient.Execute(true, []string{"--config", s.signalCliConfig, "-a", number, "block", "-g", groupId}, "")
	}
	return err
}

func (s *SignalClient) JoinGroup(number string, groupId string) error {
	var err error
	if s.signalCliMode == JsonRpc {
		type Request struct {
			GroupId string `json:"groupId"`
		}
		request := Request{GroupId: groupId}
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("updateGroup", &number, request)
	} else {
		_, err = s.cliClient.Execute(true, []string{"--config", s.signalCliConfig, "-a", number, "updateGroup", "-g", groupId}, "")
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
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("quitGroup", &number, request)
	} else {
		_, err = s.cliClient.Execute(true, []string{"--config", s.signalCliConfig, "-a", number, "quitGroup", "-g", groupId}, "")
	}
	return err
}

func (s *SignalClient) UpdateGroup(number string, groupId string, base64Avatar *string, groupDescription *string, groupName *string) error {
	var err error
	var avatarTmpPath string = ""
	if base64Avatar != nil {
		u, err := uuid.NewV4()
		if err != nil {
			return err
		}

		avatarBytes, err := base64.StdEncoding.DecodeString(*base64Avatar)
		if err != nil {
			return errors.New("Couldn't decode base64 encoded avatar: " + err.Error())
		}

		fType, err := filetype.Get(avatarBytes)
		if err != nil {
			return err
		}

		avatarTmpPath = s.avatarTmpDir + u.String() + "." + fType.Extension

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
			GroupId     string  `json:"groupId"`
			Avatar      string  `json:"avatar,omitempty"`
			Description *string `json:"description,omitempty"`
			Name        *string `json:"name,omitempty"`
		}
		request := Request{GroupId: groupId}

		if base64Avatar != nil {
			request.Avatar = avatarTmpPath
		}

		request.Description = groupDescription
		request.Name = groupName

		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("updateGroup", &number, request)
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", number, "updateGroup", "-g", groupId}
		if base64Avatar != nil {
			cmd = append(cmd, []string{"-a", avatarTmpPath}...)
		}

		if groupDescription != nil {
			cmd = append(cmd, []string{"-d", *groupDescription}...)
		}

		if groupName != nil {
			cmd = append(cmd, []string{"-n", *groupName}...)
		}

		_, err = s.cliClient.Execute(true, cmd, "")
	}

	if avatarTmpPath != "" {
		cleanupTmpFiles([]string{avatarTmpPath})
	}

	return err
}

func (s *SignalClient) SendReaction(number string, recipient string, emoji string, target_author string, timestamp int64, remove bool) error {
	// see https://github.com/AsamK/signal-cli/blob/master/man/signal-cli.1.adoc#sendreaction
	var err error
	recp := recipient
	isGroup := false
	if strings.HasPrefix(recipient, groupPrefix) {
		isGroup = true
		recp, err = ConvertGroupIdToInternalGroupId(recipient)
		if err != nil {
			return errors.New("Invalid group id")
		}
	}
	if remove && emoji == "" {
		emoji = "👍" // emoji must not be empty to remove a reaction
	}

	if s.signalCliMode == JsonRpc {
		type Request struct {
			Recipient    string `json:"recipient,omitempty"`
			GroupId      string `json:"group-id,omitempty"`
			Emoji        string `json:"emoji"`
			TargetAuthor string `json:"target-author"`
			Timestamp    int64  `json:"target-timestamp"`
			Remove       bool   `json:"remove,omitempty"`
		}
		request := Request{}
		if !isGroup {
			request.Recipient = recp
		} else {
			request.GroupId = recp
		}
		request.Emoji = emoji
		request.TargetAuthor = target_author
		request.Timestamp = timestamp
		if remove {
			request.Remove = remove
		}
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("sendReaction", &number, request)
		return err
	}

	cmd := []string{
		"--config", s.signalCliConfig,
		"-a", number,
		"sendReaction",
	}
	if !isGroup {
		cmd = append(cmd, recp)
	} else {
		cmd = append(cmd, []string{"-g", recp}...)
	}
	cmd = append(cmd, []string{"-e", emoji, "-a", target_author, "-t", strconv.FormatInt(timestamp, 10)}...)
	if remove {
		cmd = append(cmd, "-r")
	}
	_, err = s.cliClient.Execute(true, cmd, "")
	return err
}

func (s *SignalClient) SendReceipt(number string, recipient string, receipt_type string, timestamp int64) error {
	// see https://github.com/AsamK/signal-cli/blob/master/man/signal-cli.1.adoc#sendreceipt
	var err error
	recp := recipient

	if s.signalCliMode == JsonRpc {
		type Request struct {
			Recipient   string `json:"recipient,omitempty"`
			ReceiptType string `json:"receipt-type"`
			Timestamp   int64  `json:"target-timestamp"`
		}
		request := Request{}
		request.Recipient = recp
		request.ReceiptType = receipt_type
		request.Timestamp = timestamp

		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("sendReceipt", &number, request)
		return err
	}

	cmd := []string{
		"--config", s.signalCliConfig,
		"-a", number,
		"sendReceipt",
		recp,
	}

	cmd = append(cmd, []string{"-t", strconv.FormatInt(timestamp, 10)}...)

	_, err = s.cliClient.Execute(true, cmd, "")
	return err
}

func (s *SignalClient) SendStartTyping(number string, recipient string) error {
	var err error
	recp := recipient
	isGroup := false
	if strings.HasPrefix(recipient, groupPrefix) {
		isGroup = true
		recp, err = ConvertGroupIdToInternalGroupId(recipient)
		if err != nil {
			return errors.New("Invalid group id")
		}
	}

	if s.signalCliMode == JsonRpc {
		type Request struct {
			Recipient string `json:"recipient,omitempty"`
			GroupId   string `json:"group-id,omitempty"`
		}
		request := Request{}
		if !isGroup {
			request.Recipient = recp
		} else {
			request.GroupId = recp
		}

		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("sendTyping", &number, request)
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", number, "sendTyping"}
		if !isGroup {
			cmd = append(cmd, recp)
		} else {
			cmd = append(cmd, []string{"-g", recp}...)
		}
		_, err = s.cliClient.Execute(true, cmd, "")
	}

	return err
}

func (s *SignalClient) SendStopTyping(number string, recipient string) error {
	var err error
	recp := recipient
	isGroup := false
	if strings.HasPrefix(recipient, groupPrefix) {
		isGroup = true
		recp, err = ConvertGroupIdToInternalGroupId(recipient)
		if err != nil {
			return errors.New("Invalid group id")
		}
	}

	if s.signalCliMode == JsonRpc {
		type Request struct {
			Recipient string `json:"recipient,omitempty"`
			GroupId   string `json:"group-id,omitempty"`
			Stop      bool   `json:"stop"`
		}
		request := Request{Stop: true}
		if !isGroup {
			request.Recipient = recp
		} else {
			request.GroupId = recp
		}

		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("sendTyping", &number, request)
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", number, "sendTyping", "--stop"}
		if !isGroup {
			cmd = append(cmd, recp)
		} else {
			cmd = append(cmd, []string{"-g", recp}...)
		}
		_, err = s.cliClient.Execute(true, cmd, "")
	}

	return err
}

func (s *SignalClient) SearchForNumbers(number string, numbers []string) ([]SearchResultEntry, error) {
	searchResultEntries := []SearchResultEntry{}

	var err error
	var rawData string
	if s.signalCliMode == JsonRpc {
		type Request struct {
			Numbers []string `json:"recipient"`
		}
		request := Request{Numbers: numbers}

		jsonRpc2Clients := s.getJsonRpc2Clients()
		if len(jsonRpc2Clients) == 0 {
			return searchResultEntries, errors.New("No JsonRpc2Client registered!")
		}
		for _, jsonRpc2Client := range jsonRpc2Clients {
			rawData, err = jsonRpc2Client.getRaw("getUserStatus", &number, request)
			if err == nil { //getUserStatus doesn't need an account to work, so try all the registered acounts and stop until we succeed
				break
			}
		}

		if err != nil {
			return searchResultEntries, err
		}
	} else {
		cmd := []string{"--config", s.signalCliConfig, "--output", "json"}
		if number != "" {
			cmd = append(cmd, []string{"-a", number}...)
		}
		cmd = append(cmd, "getUserStatus")
		cmd = append(cmd, numbers...)
		rawData, err = s.cliClient.Execute(true, cmd, "")
	}

	if err != nil {
		return searchResultEntries, err
	}

	type SignalCliResponse struct {
		Number       string `json:"number"`
		IsRegistered bool   `json:"isRegistered"`
	}

	var resp []SignalCliResponse
	err = json.Unmarshal([]byte(rawData), &resp)
	if err != nil {
		return searchResultEntries, err
	}

	for _, val := range resp {
		searchResultEntry := SearchResultEntry{Number: val.Number, Registered: val.IsRegistered}
		searchResultEntries = append(searchResultEntries, searchResultEntry)
	}

	return searchResultEntries, err
}

func (s *SignalClient) SendContacts(number string) error {
	var err error
	if s.signalCliMode == JsonRpc {
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("sendContacts", &number, nil)
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", number, "sendContacts"}
		_, err = s.cliClient.Execute(true, cmd, "")
	}
	return err
}

func (s *SignalClient) UpdateContact(number string, recipient string, name *string, expirationInSeconds *int) error {
	var err error
	if s.signalCliMode == JsonRpc {
		type Request struct {
			Recipient  string `json:"recipient"`
			Name       string `json:"name,omitempty"`
			Expiration int    `json:"expiration,omitempty"`
		}
		request := Request{Recipient: recipient}
		if name != nil {
			request.Name = *name
		}
		if expirationInSeconds != nil {
			request.Expiration = *expirationInSeconds
		}
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("updateContact", &number, request)
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", number, "updateContact", recipient}
		if name != nil {
			cmd = append(cmd, []string{"-n", *name}...)
		}
		if expirationInSeconds != nil {
			cmd = append(cmd, []string{"-e", strconv.Itoa(*expirationInSeconds)}...)
		}
		_, err = s.cliClient.Execute(true, cmd, "")
	}
	return err
}

func (s *SignalClient) AddDevice(number string, uri string) error {
	var err error
	if s.signalCliMode == JsonRpc {
		type Request struct {
			Uri string `json:"uri"`
		}
		request := Request{Uri: uri}
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("addDevice", &number, request)
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", number, "addDevice", "--uri", uri}
		_, err = s.cliClient.Execute(true, cmd, "")
	}
	return err
}

func (s *SignalClient) SetTrustMode(number string, trustMode utils.SignalCliTrustMode) error {
	s.signalCliApiConfig.SetTrustModeForNumber(number, trustMode)
	return s.signalCliApiConfig.Persist()
}

func (s *SignalClient) GetTrustMode(number string) utils.SignalCliTrustMode {
	trustMode, err := s.signalCliApiConfig.GetTrustModeForNumber(number)
	if err != nil { //no trust mode explicitly set, use signal-cli default
		return utils.OnFirstUseTrust
	}
	return trustMode
}

func (s *SignalClient) SubmitRateLimitChallenge(number string, challengeToken string, captcha string) error {
	if s.signalCliMode == JsonRpc {
		type Request struct {
			Challenge string `json:"challenge"`
			Captcha   string `json:"captcha"`
		}
		request := Request{Challenge: challengeToken, Captcha: captcha}
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("submitRateLimitChallenge", &number, request)
		return err
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", number, "submitRateLimitChallenge", "--challenge", challengeToken, "--captcha", captcha}
		_, err := s.cliClient.Execute(true, cmd, "")
		return err
	}
}

func (s *SignalClient) SetUsername(number string, username string) (SetUsernameResponse, error) {
	type SetUsernameSignalCliResponse struct {
		Username     string `json:"username"`
		UsernameLink string `json:"usernameLink"`
	}

	var resp SetUsernameResponse
	var err error
	var rawData string
	if s.signalCliMode == JsonRpc {
		type Request struct {
			Username string `json:"username"`
		}
		request := Request{Username: username}
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return resp, err
		}
		rawData, err = jsonRpc2Client.getRaw("updateAccount", &number, request)
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-o", "json", "-a", number, "updateAccount", "-u", username}
		rawData, err = s.cliClient.Execute(true, cmd, "")
	}

	var signalCliResp SetUsernameSignalCliResponse
	err = json.Unmarshal([]byte(rawData), &signalCliResp)
	if err != nil {
		return resp, errors.New("Couldn't process request - invalid signal-cli response")
	}

	resp.Username = signalCliResp.Username
	resp.UsernameLink = signalCliResp.UsernameLink

	return resp, err
}

func (s *SignalClient) RemoveUsername(number string) error {
	if s.signalCliMode == JsonRpc {
		type Request struct {
			DeleteUsername bool `json:"delete-username"`
		}
		request := Request{DeleteUsername: true}
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("updateAccount", &number, request)
		return err
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-o", "json", "-a", number, "updateAccount", "--delete-username"}
		_, err := s.cliClient.Execute(true, cmd, "")
		return err
	}
}

func (s *SignalClient) UpdateAccountSettings(number string, discoverableByNumber *bool, shareNumber *bool) error {
	if s.signalCliMode == JsonRpc {
		type Request struct {
			ShareNumber          *bool `json:"number-sharing"`
			DiscoverableByNumber *bool `json:"discoverable-by-number"`
		}
		request := Request{}
		request.DiscoverableByNumber = discoverableByNumber
		request.ShareNumber = shareNumber

		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("updateAccount", &number, request)
		return err
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-a", number, "updateAccount"}
		if discoverableByNumber != nil {
			cmd = append(cmd, []string{"--discoverable-by-number", strconv.FormatBool(*discoverableByNumber)}...)
		}

		if shareNumber != nil {
			cmd = append(cmd, []string{"--number-sharing", strconv.FormatBool(*shareNumber)}...)
		}
		_, err := s.cliClient.Execute(true, cmd, "")
		return err
	}
}

func (s *SignalClient) ListInstalledStickerPacks(number string) ([]ListInstalledStickerPacksResponse, error) {
	type ListInstalledStickerPacksSignalCliResponse struct {
		PackId    string `json:"packId"`
		Url       string `json:"url"`
		Installed bool   `json:"installed"`
		Title     string `json:"title"`
		Author    string `json:"author"`
	}

	resp := []ListInstalledStickerPacksResponse{}

	var err error
	var rawData string
	if s.signalCliMode == JsonRpc {
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return resp, err
		}
		rawData, err = jsonRpc2Client.getRaw("listStickerPacks", &number, nil)
		if err != nil {
			return resp, err
		}
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-o", "json", "-a", number, "listStickerPacks"}
		rawData, err = s.cliClient.Execute(true, cmd, "")
		if err != nil {
			return resp, err
		}
	}

	var signalCliResp []ListInstalledStickerPacksSignalCliResponse
	err = json.Unmarshal([]byte(rawData), &signalCliResp)
	if err != nil {
		return resp, errors.New("Couldn't process request - invalid signal-cli response")
	}

	for _, value := range signalCliResp {
		resp = append(resp, ListInstalledStickerPacksResponse{PackId: value.PackId, Url: value.Url,
			Installed: value.Installed, Title: value.Title, Author: value.Author})
	}

	return resp, nil
}

func (s *SignalClient) AddStickerPack(number string, packId string, packKey string) error {

	stickerPackUri := fmt.Sprintf(`https://signal.art/addstickers/#pack_id=%s&pack_key=%s`, packId, packKey)

	if s.signalCliMode == JsonRpc {
		type Request struct {
			Uri string `json:"uri"`
		}
		request := Request{Uri: stickerPackUri}
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return err
		}
		_, err = jsonRpc2Client.getRaw("addStickerPack", &number, request)
		return err
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-o", "json", "-a", number, "addStickerPack", "--uri", stickerPackUri}
		_, err := s.cliClient.Execute(true, cmd, "")
		return err
	}
}

func (s *SignalClient) ListContacts(number string) ([]ListContactsResponse, error) {
	type ListContactsSignlCliResponse struct {
		Number            string `json:"number"`
		Uuid              string `json:"uuid"`
		Name              string `json:"name"`
		ProfileName       string `json:"profileName"`
		Username          string `json:"username"`
		Color             string `json:"color"`
		Blocked           bool   `json:"blocked"`
		MessageExpiration string `json:"messageExpiration"`
	}

	resp := []ListContactsResponse{}

	var err error
	var rawData string

	if s.signalCliMode == JsonRpc {
		jsonRpc2Client, err := s.getJsonRpc2Client()
		if err != nil {
			return nil, err
		}
		rawData, err = jsonRpc2Client.getRaw("listContacts", &number, nil)
		if err != nil {
			return resp, err
		}
	} else {
		cmd := []string{"--config", s.signalCliConfig, "-o", "json", "-a", number, "listContacts"}
		rawData, err = s.cliClient.Execute(true, cmd, "")
		if err != nil {
			return resp, err
		}
	}

	var signalCliResp []ListContactsSignlCliResponse
	err = json.Unmarshal([]byte(rawData), &signalCliResp)
	if err != nil {
		return resp, errors.New("Couldn't process request - invalid signal-cli response")
	}

	for _, value := range signalCliResp {
		resp = append(resp, ListContactsResponse{
			Number:            value.Number,
			Uuid:              value.Uuid,
			Name:              value.Name,
			ProfileName:       value.ProfileName,
			Username:          value.Username,
			Color:             value.Color,
			Blocked:           value.Blocked,
			MessageExpiration: value.MessageExpiration,
		})
	}

	return resp, nil
}
