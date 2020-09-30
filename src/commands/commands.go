package commands

import (
	datastructures "github.com/bbernhard/signal-cli-rest-api/datastructures"
	system "github.com/bbernhard/signal-cli-rest-api/system"
	"strings"
	"encoding/base64"
	"github.com/h2non/filetype"
	"github.com/gin-gonic/gin"
	uuid "github.com/gofrs/uuid"
	"os"
	log "github.com/sirupsen/logrus"
	qrcode "github.com/skip2/go-qrcode"
)

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

func ConvertInternalGroupIdToGroupId(internalId string) string {
	return datastructures.GroupPrefix + base64.StdEncoding.EncodeToString([]byte(internalId))
}

func GetGroups(signalCliConfig string, number string) ([]datastructures.GroupEntry, error) {
	groupEntries := []datastructures.GroupEntry{}

	out, err := system.RunSignalCli(true, []string{"--config", signalCliConfig, "-u", number, "listGroups", "-d"})
	if err != nil {
		return groupEntries, err
	}

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		var groupEntry datastructures.GroupEntry
		if line == "" {
			continue
		}

		idIdx := strings.Index(line, " Name: ")
		idPair := line[:idIdx]
		groupEntry.InternalId = strings.TrimPrefix(idPair, "Id: ")
		groupEntry.Id = ConvertInternalGroupIdToGroupId(groupEntry.InternalId)
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


func sendMessageViaDbus(c *gin.Context, attachmentTmpDir string, signalCliConfig string, number string, message string,
	recipients []string, base64Attachments []string, isGroup bool) {

	messageType := "org.asamk.Signal.sendMessage"
	if isGroup {
		messageType = "org.asamk.Signal.sendGroupMessage"
	}

	cmd := []string{"--system", "--type=method_call", "--print-reply", "--dest=org.asamk.Signal", 
						"/org/asamk/Signal", messageType, "string:" + message }


	if len(recipients) == 0 {
		c.JSON(400, gin.H{"error": "Please specify at least one recipient"})
		return
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
			system.CleanupTmpFiles(attachmentTmpPaths)
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := f.Sync(); err != nil {
			system.CleanupTmpFiles(attachmentTmpPaths)
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		f.Close()
	}

	delimitedAttachmentPaths := "array:string:"
	if len(attachmentTmpPaths) > 0 {
		joinedAttachmentTmpPaths := strings.Join(attachmentTmpPaths, ",")
		delimitedAttachmentPaths += joinedAttachmentTmpPaths
	}
	cmd = append(cmd, delimitedAttachmentPaths)

	delimitedRecipients := "array:string:"
	if !isGroup {
		joinedRecipients := strings.Join(recipients, ",")
		delimitedRecipients += joinedRecipients
		cmd = append(cmd, delimitedRecipients)
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

		groupIdByteArray := "array:byte:"

		groupIdArray := []string{}
		for _, c := range groupId {
			groupIdArray = append(groupIdArray, string(c))
		}
		groupIdByteArray += strings.Join(groupIdArray, ",")

		cmd = append(cmd, groupIdByteArray)
	}

	log.Info(cmd)

	_, err := system.RunCommand("dbus-send", cmd)
	if err != nil {
		//system.CleanupTmpFiles(attachmentTmpPaths)
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	system.CleanupTmpFiles(attachmentTmpPaths)
	c.JSON(201, nil)
}

func sendMessage(c *gin.Context, attachmentTmpDir string, signalCliConfig string, number string, message string,
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
			system.CleanupTmpFiles(attachmentTmpPaths)
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := f.Sync(); err != nil {
			system.CleanupTmpFiles(attachmentTmpPaths)
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		f.Close()
	}

	if len(attachmentTmpPaths) > 0 {
		cmd = append(cmd, "-a")
		cmd = append(cmd, attachmentTmpPaths...)
	}

	_, err := system.RunSignalCli(true, cmd)
	if err != nil {
		system.CleanupTmpFiles(attachmentTmpPaths)
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	system.CleanupTmpFiles(attachmentTmpPaths)
	c.JSON(201, nil)
}


func SendMessage(c *gin.Context, attachmentTmpDir string, signalCliConfig string, number string, message string,
	recipients []string, base64Attachments []string, isGroup bool) {

	if system.UseDbus() {
		sendMessageViaDbus(c, attachmentTmpDir, signalCliConfig, number, message, recipients, base64Attachments, isGroup)
		return
	}

	sendMessage(c, attachmentTmpDir, signalCliConfig, number, message, recipients, base64Attachments, isGroup)
	
}

func RegisterNumber(signalCliConfig string, number string, useVoice bool) error {
	command := []string{"--config", signalCliConfig, "-u", number, "register"}

	if useVoice == true {
		command = append(command, "--voice")
	}

	_, err := system.RunSignalCli(true, command)
	if err != nil {
		return err
	}
	return nil
}

func VerifyRegisteredNumber(signalCliConfig string, number string, token string, pin string) error {
	cmd := []string{"--config", signalCliConfig, "-u", number, "verify", token}
	if pin != "" {
		cmd = append(cmd, "--pin")
		cmd = append(cmd, pin)
	}

	_, err := system.RunSignalCli(true, cmd)
	if err != nil {
		return err
	}
	return nil
}

func CreateGroup(signalCliConfig string, number string, groupName string, groupMembers []string) (string, error) {
	cmd := []string{"--config", signalCliConfig, "-u", number, "updateGroup", "-n", groupName, "-m"}
	cmd = append(cmd, groupMembers...)

	out, err := system.RunSignalCli(true, cmd)
	if err != nil {
		return "", err
	}

	internalGroupId := getStringInBetween(out, `"`, `"`)
	return internalGroupId, nil
}

func Receive(signalCliConfig string, number string,) (string, error) {
	command := []string{"--config", signalCliConfig, "-u", number, "receive", "-t", "1", "--json"}
	out, err := system.RunSignalCli(true, command)
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

func DeleteGroup(signalCliConfig string, number string, groupId string) error {
	_, err := system.RunSignalCli(true, []string{"--config", signalCliConfig, "-u", number, "quitGroup", "-g", groupId})
	if err != nil {
		return err
	}

	return nil
}

func GetQrCodeLink(signalCliConfig string, deviceName string) ([]byte, error) {
	command := []string{"--config", signalCliConfig, "link", "-n", deviceName}

	var png []byte

	tsdeviceLink, err := system.RunSignalCli(false, command)
	if err != nil {
		return png, err
	}

	q, err := qrcode.New(string(tsdeviceLink), qrcode.Medium)
	if err != nil {
		return png, err
	}

	q.DisableBorder = true
	png, err = q.PNG(256)
	if err != nil {
		return png, err
	}

	return png, nil
}
