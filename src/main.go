package main

import (
	"encoding/json"
	"flag"
	"github.com/bbernhard/signal-cli-rest-api/api"
	"github.com/bbernhard/signal-cli-rest-api/client"
	docs "github.com/bbernhard/signal-cli-rest-api/docs"
	"github.com/bbernhard/signal-cli-rest-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
)

// @title Signal Cli REST API
// @version 1.0
// @description This is the Signal Cli REST API documentation.

// @tag.name General
// @tag.description Some general endpoints.

// @tag.name Devices
// @tag.description Register and link Devices.

// @tag.name Accounts
// @tag.description List registered and linked accounts

// @tag.name Groups
// @tag.description Create, List and Delete Signal Groups.

// @tag.name Messages
// @tag.description Send and Receive Signal Messages.

// @tag.name Attachments
// @tag.description List and Delete Attachments.

// @tag.name Profiles
// @tag.description Update Profile.

// @tag.name Identities
// @tag.description List and Trust Identities.

// @tag.name Reactions
// @tag.description React to messages.

// @tag.name Receipts
// @tag.description Send receipts for messages.

// @tag.name Search
// @tag.description Search the Signal Service.

// @tag.name Sticker Packs
// @tag.description List and Install Sticker Packs

// @host localhost:8080
// @schemes http
// @BasePath /
func main() {
	signalCliConfig := flag.String("signal-cli-config", "/home/.local/share/signal-cli/", "Config directory where signal-cli config is stored")
	attachmentTmpDir := flag.String("attachment-tmp-dir", "/tmp/", "Attachment tmp directory")
	avatarTmpDir := flag.String("avatar-tmp-dir", "/tmp/", "Avatar tmp directory")
	flag.Parse()

	docs.SwaggerInfo.Schemes = []string{"http", "https"}

	router := gin.New()
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/v1/health"}, //do not log the health requests (to avoid spamming the log file)
	}))

	router.Use(gin.Recovery())

	port := utils.GetEnv("PORT", "8080")
	if _, err := strconv.Atoi(port); err != nil {
		log.Fatal("Invalid PORT ", port, " set. PORT needs to be a number")
	}

	defaultSwaggerIp := utils.GetEnv("HOST_IP", "127.0.0.1")
	swaggerIp := utils.GetEnv("SWAGGER_IP", defaultSwaggerIp)
	swaggerHost := utils.GetEnv("SWAGGER_HOST", swaggerIp+":"+port)
	docs.SwaggerInfo.Host = swaggerHost

	log.Info("Started Signal Messenger REST API")

	supportsSignalCliNative := "0"
	if _, err := os.Stat("/usr/bin/signal-cli-native"); err == nil {
		supportsSignalCliNative = "1"
	}

	err := os.Setenv("SUPPORTS_NATIVE", supportsSignalCliNative)
	if err != nil {
		log.Fatal("Couldn't set env variable: ", err.Error())
	}

	useNative := utils.GetEnv("USE_NATIVE", "")
	if useNative != "" {
		log.Warning("The env variable USE_NATIVE is deprecated. Please use the env variable MODE instead")
	}

	signalCliMode := client.Normal
	mode := utils.GetEnv("MODE", "normal")
	if mode == "normal" {
		signalCliMode = client.Normal
	} else if mode == "json-rpc" {
		signalCliMode = client.JsonRpc
	} else if mode == "native" {
		signalCliMode = client.Native
	}

	if useNative != "" {
		_, modeEnvVariableSet := os.LookupEnv("MODE")
		if modeEnvVariableSet {
			log.Fatal("You have both the USE_NATIVE and the MODE env variable set. Please remove the deprecated env variable USE_NATIVE!")
		}
	}

	if useNative == "1" || signalCliMode == client.Native {
		if supportsSignalCliNative == "0" {
			log.Error("signal-cli-native is not support on this system...falling back to signal-cli")
			signalCliMode = client.Normal
		}
	}

	if signalCliMode == client.JsonRpc {
		_, autoReceiveScheduleEnvVariableSet := os.LookupEnv("AUTO_RECEIVE_SCHEDULE")
		if autoReceiveScheduleEnvVariableSet {
			log.Fatal("Env variable AUTO_RECEIVE_SCHEDULE can't be used with mode json-rpc")
		}

		_, signalCliCommandTimeoutEnvVariableSet := os.LookupEnv("SIGNAL_CLI_CMD_TIMEOUT")
		if signalCliCommandTimeoutEnvVariableSet {
			log.Fatal("Env variable SIGNAL_CLI_CMD_TIMEOUT can't be used with mode json-rpc")
		}
	}

	jsonRpc2ClientConfigPathPath := *signalCliConfig + "/jsonrpc2.yml"
	signalCliApiConfigPath := *signalCliConfig + "/api-config.yml"
	signalClient := client.NewSignalClient(*signalCliConfig, *attachmentTmpDir, *avatarTmpDir, signalCliMode, jsonRpc2ClientConfigPathPath, signalCliApiConfigPath)
	err = signalClient.Init()
	if err != nil {
		log.Fatal("Couldn't init Signal Client: ", err.Error())
	}

	api := api.NewApi(signalClient)
	v1 := router.Group("/v1")
	{
		about := v1.Group("/about")
		{
			about.GET("", api.About)
		}

		configuration := v1.Group("/configuration")
		{
			configuration.GET("", api.GetConfiguration)
			configuration.POST("", api.SetConfiguration)
			configuration.POST(":number/settings", api.SetTrustMode)
			configuration.GET(":number/settings", api.GetTrustMode)
		}

		health := v1.Group("/health")
		{
			health.GET("", api.Health)
		}

		register := v1.Group("/register")
		{
			register.POST(":number", api.RegisterNumber)
			register.POST(":number/verify/:token", api.VerifyRegisteredNumber)
		}

		unregister := v1.Group("unregister")
		{
			unregister.POST(":number", api.UnregisterNumber)
		}

		sendV1 := v1.Group("/send")
		{
			sendV1.POST("", api.Send)
		}

		receive := v1.Group("/receive")
		{
			receive.GET(":number", api.Receive)
		}

		groups := v1.Group("/groups")
		{
			groups.POST(":number", api.CreateGroup)
			groups.GET(":number", api.GetGroups)
			groups.GET(":number/:groupid", api.GetGroup)
			groups.DELETE(":number/:groupid", api.DeleteGroup)
			groups.POST(":number/:groupid/block", api.BlockGroup)
			groups.POST(":number/:groupid/join", api.JoinGroup)
			groups.POST(":number/:groupid/quit", api.QuitGroup)
			groups.PUT(":number/:groupid", api.UpdateGroup)
			groups.POST(":number/:groupid/members", api.AddMembersToGroup)
			groups.DELETE(":number/:groupid/members", api.RemoveMembersFromGroup)
			groups.POST(":number/:groupid/admins", api.AddAdminsToGroup)
			groups.DELETE(":number/:groupid/admins", api.RemoveAdminsFromGroup)
		}

		link := v1.Group("qrcodelink")
		{
			link.GET("", api.GetQrCodeLink)
		}

		accounts := v1.Group("accounts")
		{
			accounts.GET("", api.GetAccounts)
			accounts.POST(":number/rate-limit-challenge", api.SubmitRateLimitChallenge)
			accounts.PUT(":number/settings", api.UpdateAccountSettings)
			accounts.POST(":number/username", api.SetUsername)
			accounts.DELETE(":number/username", api.RemoveUsername)
		}

		devices := v1.Group("devices")
		{
			devices.POST(":number", api.AddDevice)
		}

		attachments := v1.Group("attachments")
		{
			attachments.GET("", api.GetAttachments)
			attachments.DELETE(":attachment", api.RemoveAttachment)
			attachments.GET(":attachment", api.ServeAttachment)
		}

		stickerPacks := v1.Group("sticker-packs")
		{
			stickerPacks.GET(":number", api.ListInstalledStickerPacks)
			stickerPacks.POST(":number", api.AddStickerPack)
		}

		profiles := v1.Group("profiles")
		{
			profiles.PUT(":number", api.UpdateProfile)
		}

		identities := v1.Group("identities")
		{
			identities.GET(":number", api.ListIdentities)
			identities.PUT(":number/trust/:numbertotrust", api.TrustIdentity)
		}

		typingIndicator := v1.Group("typing-indicator")
		{
			typingIndicator.PUT(":number", api.SendStartTyping)
			typingIndicator.DELETE(":number", api.SendStopTyping)
		}

		reactions := v1.Group("/reactions")
		{
			reactions.POST(":number", api.SendReaction)
			reactions.DELETE(":number", api.RemoveReaction)
		}

		receipts := v1.Group("/receipts")
		{
			receipts.POST(":number", api.SendReceipt)
		}

		search := v1.Group("/search")
		{
			search.GET("", api.SearchForNumbers)
			search.GET(":number", api.SearchForNumbers)
		}

		contacts := v1.Group("/contacts")
		{
			contacts.GET(":number", api.ListContacts)
			contacts.PUT(":number", api.UpdateContact)
			contacts.POST(":number/sync", api.SendContacts)
		}
	}

	v2 := router.Group("/v2")
	{
		sendV2 := v2.Group("/send")
		{
			sendV2.POST("", api.SendV2)
		}
	}

	swaggerUrl := ginSwagger.URL("http://" + swaggerIp + ":" + string(port) + "/swagger/doc.json")
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, swaggerUrl))

	autoReceiveSchedule := utils.GetEnv("AUTO_RECEIVE_SCHEDULE", "")
	if autoReceiveSchedule != "" {
		p := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := p.Parse(autoReceiveSchedule)
		if err != nil {
			log.Fatal("AUTO_RECEIVE_SCHEDULE: Invalid schedule: ", err.Error())
		}

		type SignalCliAccountConfig struct {
			Number string `json:"number"`
		}

		type SignalCliAccountConfigs struct {
			Accounts []SignalCliAccountConfig `json:"accounts"`
		}

		autoReceiveScheduleReceiveTimeout := utils.GetEnv("AUTO_RECEIVE_SCHEDULE_RECEIVE_TIMEOUT", "10")
		autoReceiveScheduleIgnoreAttachments := utils.GetEnv("AUTO_RECEIVE_SCHEDULE_IGNORE_ATTACHMENTS", "false")
		autoReceiveScheduleIgnoreStories := utils.GetEnv("AUTO_RECEIVE_SCHEDULE_IGNORE_STORIES", "false")
		autoReceiveScheduleSendReadReceipts := utils.GetEnv("AUTO_RECEIVE_SCHEDULE_SEND_READ_RECEIPTS", "false")

		c := cron.New()
		c.Schedule(schedule, cron.FuncJob(func() {
			accountsJsonPath := *signalCliConfig + "/data/accounts.json"
			if _, err := os.Stat(accountsJsonPath); err == nil {
				signalCliConfigJsonData, err := ioutil.ReadFile(accountsJsonPath)
				if err != nil {
					log.Fatal("AUTO_RECEIVE_SCHEDULE: Couldn't read accounts.json: ", err.Error())
				}
				var signalCliAccountConfigs SignalCliAccountConfigs
				err = json.Unmarshal(signalCliConfigJsonData, &signalCliAccountConfigs)
				if err != nil {
					log.Fatal("AUTO_RECEIVE_SCHEDULE: Couldn't parse accounts.json: ", err.Error())
				}

				for _, account := range signalCliAccountConfigs.Accounts {
					client := &http.Client{}

					log.Debug("AUTO_RECEIVE_SCHEDULE: Calling receive for number ", account.Number)
					req, err := http.NewRequest("GET", "http://127.0.0.1:"+port+"/v1/receive/"+account.Number, nil)
					if err != nil {
						log.Error("AUTO_RECEIVE_SCHEDULE: Couldn't call receive for number ", account.Number, ": ", err.Error())
					}

					q := req.URL.Query()
					q.Add("timeout", autoReceiveScheduleReceiveTimeout)
					q.Add("ignore_attachments", autoReceiveScheduleIgnoreAttachments)
					q.Add("ignore_stories", autoReceiveScheduleIgnoreStories)
					q.Add("send_read_receipts", autoReceiveScheduleSendReadReceipts)
					req.URL.RawQuery = q.Encode()

					resp, err := client.Do(req)
					if err != nil {
						log.Error("AUTO_RECEIVE_SCHEDULE: Couldn't call receive for number ", account.Number, ": ", err.Error())
					}

					if resp.StatusCode != 200 {
						jsonResp, err := ioutil.ReadAll(resp.Body)
						resp.Body.Close()
						if err != nil {
							log.Error("AUTO_RECEIVE_SCHEDULE: Couldn't read json response: ", err.Error())
							continue
						}

						type ReceiveResponse struct {
							Error string `json:"error"`
						}
						var receiveResponse ReceiveResponse
						err = json.Unmarshal(jsonResp, &receiveResponse)
						if err != nil {
							log.Error("AUTO_RECEIVE_SCHEDULE: Couldn't parse json response: ", err.Error())
							continue
						}

						log.Error("AUTO_RECEIVE_SCHEDULE: Couldn't call receive for number ", account.Number, ": ", receiveResponse)
					}
				}
			} else {
				log.Info("AUTO_RECEIVE_SCHEDULE: accounts.json doesn't exist")
			}
		}))
		c.Start()
	}

	router.Run()
}
