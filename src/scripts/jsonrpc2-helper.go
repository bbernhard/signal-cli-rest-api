package main

import (
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/bbernhard/signal-cli-rest-api/utils"
	log "github.com/sirupsen/logrus"
)

func main() {
	signalCliConfigDir := "/home/.local/share/signal-cli/"
	signalCliConfigDirEnv := utils.GetEnv("SIGNAL_CLI_CONFIG_DIR", "")
	if signalCliConfigDirEnv != "" {
		signalCliConfigDir = signalCliConfigDirEnv
		if !strings.HasSuffix(signalCliConfigDirEnv, "/") {
			signalCliConfigDir += "/"
		}
	}

	jsonRpc2ClientConfig := utils.NewJsonRpc2ClientConfig()

	var tcpPort int64 = 6001
	jsonRpc2ClientConfig.AddEntry(utils.MULTI_ACCOUNT_NUMBER, utils.JsonRpc2ClientConfigEntry{TcpPort: tcpPort})

	args := []string{"--output=json", "--config", signalCliConfigDir}

	trustNewIdentitiesEnv := utils.GetEnv("JSON_RPC_TRUST_NEW_IDENTITIES", "")
	if trustNewIdentitiesEnv == "on-first-use" {
		args = append(args, []string{"--trust-new-identities", "on-first-use"}...)
	} else if trustNewIdentitiesEnv == "always" {
		args = append(args, []string{"--trust-new-identities", "always"}...)
	} else if trustNewIdentitiesEnv == "never" {
		args = append(args, []string{"--trust-new-identities", "never"}...)
	} else if trustNewIdentitiesEnv != "" {
		log.Fatal("Invalid JSON_RPC_TRUST_NEW_IDENTITIES environment variable set!")
	}

	args = append(args, "daemon")

	ignoreAttachments := utils.GetEnv("JSON_RPC_IGNORE_ATTACHMENTS", "")
	if ignoreAttachments == "true" {
		args = append(args, "--ignore-attachments")
	}

	ignoreStories := utils.GetEnv("JSON_RPC_IGNORE_STORIES", "")
	if ignoreStories == "true" {
		args = append(args, "--ignore-stories")
	}

	ignoreAvatars := utils.GetEnv("JSON_RPC_IGNORE_AVATARS", "")
	if ignoreAvatars == "true" {
		args = append(args, "--ignore-avatars")
	}

	ignoreStickers := utils.GetEnv("JSON_RPC_IGNORE_STICKERS", "")
	if ignoreStickers == "true" {
		args = append(args, "--ignore-stickers")
	}

	args = append(args, []string{"--tcp", "127.0.0.1:" + strconv.FormatInt(tcpPort, 10)}...)

	// write jsonrpc.yml config file
	err := jsonRpc2ClientConfig.Persist(signalCliConfigDir + "jsonrpc2.yml")
	if err != nil {
		log.Fatal("Couldn't persist jsonrpc2.yaml: ", err.Error())
	}

	log.Info("Updated jsonrpc2.yml")

	env := os.Environ()

	err = syscall.Exec("/usr/bin/signal-cli", args, env)
	if err != nil {
		log.Fatal("Couldn't start signal-cli in json-rpc mode: ", err.Error())
	}
}
