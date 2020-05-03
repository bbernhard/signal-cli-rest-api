package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"github.com/gin-gonic/gin"
	"github.com/h2non/filetype"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
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

	_, err := runSignalCli(cmd)
	if err != nil {
		cleanupTmpFiles(attachmentTmpPaths)
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, nil)
}

func getGroups(number string, signalCliConfig string) ([]GroupEntry, error) {
	groupEntries := []GroupEntry{}

	out, err := runSignalCli([]string{"--config", signalCliConfig, "-u", number, "listGroups", "-d"})
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
		groupEntry.Id = groupPrefix + base64.StdEncoding.EncodeToString([]byte(groupEntry.InternalId))
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

func runSignalCli(args []string) (string, error) {
	cmd := exec.Command("signal-cli", args...)
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
}

func main() {
	signalCliConfig := flag.String("signal-cli-config", "/home/.local/share/signal-cli/", "Config directory where signal-cli config is stored")
	attachmentTmpDir := flag.String("attachment-tmp-dir", "/tmp/", "Attachment tmp directory")
	flag.Parse()

	router := gin.Default()
	log.Info("Started Signal Messenger REST API")

	router.GET("/v1/about", func(c *gin.Context) {
		type About struct {
			SupportedApiVersions []string `json:"versions"`
			BuildNr              int      `json:"build"`
		}

		about := About{SupportedApiVersions: []string{"v1", "v2"}, BuildNr: 2}
		c.JSON(200, about)
	})

	router.POST("/v1/register/:number", func(c *gin.Context) {
		number := c.Param("number")

		type Request struct {
			UseVoice bool `json:"use_voice"`
		}

		var req Request

		buf := new(bytes.Buffer)
		buf.ReadFrom(c.Request.Body)
		if buf.String() != "" {
			err := json.Unmarshal(buf.Bytes(), &req)
			if err != nil {
				log.Error("Couldn't register number: ", err.Error())
				c.JSON(400, gin.H{"error": "Couldn't process request - invalid request."})
				return
			}
		} else {
			req.UseVoice = false
		}

		if number == "" {
			c.JSON(400, gin.H{"error": "Please provide a number"})
			return
		}

		command := []string{"--config", *signalCliConfig, "-u", number, "register"}

		if req.UseVoice == true {
			command = append(command, "--voice")
		}

		_, err := runSignalCli(command)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, nil)
	})

	router.POST("/v1/register/:number/verify/:token", func(c *gin.Context) {
		number := c.Param("number")
		token := c.Param("token")

		if number == "" {
			c.JSON(400, gin.H{"error": "Please provide a number"})
			return
		}

		if token == "" {
			c.JSON(400, gin.H{"error": "Please provide a verification code"})
			return
		}

		_, err := runSignalCli([]string{"--config", *signalCliConfig, "-u", number, "verify", token})
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, nil)
	})

	router.POST("/v1/send", func(c *gin.Context) {
		type Request struct {
			Number           string   `json:"number"`
			Recipients       []string `json:"recipients"`
			Message          string   `json:"message"`
			Base64Attachment string   `json:"base64_attachment"`
			IsGroup          bool     `json:"is_group"`
		}
		var req Request
		err := c.BindJSON(&req)
		if err != nil {
			c.JSON(400, gin.H{"error": "Couldn't process request - invalid request"})
			return
		}

		base64Attachments := []string{}
		if req.Base64Attachment != "" {
			base64Attachments = append(base64Attachments, req.Base64Attachment)
		}

		send(c, *signalCliConfig, *signalCliConfig, req.Number, req.Message, req.Recipients, base64Attachments, req.IsGroup)
	})

	router.POST("/v2/send", func(c *gin.Context) {
		type Request struct {
			Number            string   `json:"number"`
			Recipients        []string `json:"recipients"`
			Message           string   `json:"message"`
			Base64Attachments []string `json:"base64_attachments"`
		}
		var req Request
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
			send(c, *attachmentTmpDir, *signalCliConfig, req.Number, req.Message, []string{group}, req.Base64Attachments, true)
		}

		if len(recipients) > 0 {
			send(c, *attachmentTmpDir, *signalCliConfig, req.Number, req.Message, recipients, req.Base64Attachments, false)
		}
	})

	router.GET("/v1/receive/:number", func(c *gin.Context) {
		number := c.Param("number")

		command := []string{"--config", *signalCliConfig, "-u", number, "receive", "-t", "1", "--json"}
		out, err := runSignalCli(command)
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
	})

	router.POST("/v1/groups/:number", func(c *gin.Context) {
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

		cmd := []string{"--config", *signalCliConfig, "-u", number, "updateGroup", "-n", req.Name, "-m"}
		cmd = append(cmd, req.Members...)

		out, err := runSignalCli(cmd)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		
		groupId := getStringInBetween(out, `"`, `"`)
		c.JSON(201, gin.H{"id": groupPrefix + groupId})
	})

	router.GET("/v1/groups/:number", func(c *gin.Context) {
		number := c.Param("number")

		groups, err := getGroups(number, *signalCliConfig)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, groups)
	})

	router.DELETE("/v1/groups/:number/:groupid", func(c *gin.Context) {
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

		_, err = runSignalCli([]string{"--config", *signalCliConfig, "-u", number, "quitGroup", "-g", string(groupId)})
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
	})

	router.Run()
}
