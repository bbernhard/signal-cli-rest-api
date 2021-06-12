package main

import (
	"encoding/json"
	"flag"
	"strings"
	"strconv"
	"net/http"
	"os"
	"path/filepath"
	"io/ioutil"
	"github.com/bbernhard/signal-cli-rest-api/api"
	_ "github.com/bbernhard/signal-cli-rest-api/docs"
	"github.com/bbernhard/signal-cli-rest-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Signal Cli REST API
// @version 1.0
// @description This is the Signal Cli REST API documentation.

// @tag.name General
// @tag.description Some general endpoints.

// @tag.name Devices
// @tag.description Register and link Devices.

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

// @host 127.0.0.1:8080
// @BasePath /
func main() {
	signalCliConfig := flag.String("signal-cli-config", "/home/.local/share/signal-cli/", "Config directory where signal-cli config is stored")
	attachmentTmpDir := flag.String("attachment-tmp-dir", "/tmp/", "Attachment tmp directory")
	avatarTmpDir := flag.String("avatar-tmp-dir", "/tmp/", "Avatar tmp directory")
	flag.Parse()

	router := gin.New()
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/v1/health"}, //do not log the health requests (to avoid spamming the log file)
	}))

	router.Use(gin.Recovery())

	log.Info("Started Signal Messenger REST API")

	supportsSignalCliNative := "0"
	if _, err := os.Stat("/usr/bin/signal-cli-native"); err == nil {
		supportsSignalCliNative = "1"
	}

	err := os.Setenv("SUPPORTS_NATIVE", supportsSignalCliNative)
	if err != nil {
		log.Fatal("Couldn't set env variable: ", err.Error())
	}

	api := api.NewApi(*signalCliConfig, *attachmentTmpDir, *avatarTmpDir)
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
		}

		link := v1.Group("qrcodelink")
		{
			link.GET("", api.GetQrCodeLink)
		}

		attachments := v1.Group("attachments")
		{
			attachments.GET("", api.GetAttachments)
			attachments.DELETE(":attachment", api.RemoveAttachment)
			attachments.GET(":attachment", api.ServeAttachment)
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

		contacts := v1.Group("updatecontact")
		{
			contacts.POST("", api.UpdateContact)
		}
	}

	v2 := router.Group("/v2")
	{
		sendV2 := v2.Group("/send")
		{
			sendV2.POST("", api.SendV2)
		}
	}

	port := utils.GetEnv("PORT", "8080")
	if _, err := strconv.Atoi(port); err != nil {
		log.Fatal("Invalid PORT ", port, " set. PORT needs to be a number")
	}

	swaggerUrl := ginSwagger.URL("http://127.0.0.1:" + string(port) + "/swagger/doc.json")
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, swaggerUrl))

	autoReceiveSchedule := utils.GetEnv("AUTO_RECEIVE_SCHEDULE", "")
	if autoReceiveSchedule != "" {
		p := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := p.Parse(autoReceiveSchedule)
		if err != nil {
			log.Fatal("AUTO_RECEIVE_SCHEDULE: Invalid schedule: ", err.Error())
		}

		c := cron.New()
		c.Schedule(schedule, cron.FuncJob(func() {
			err := filepath.Walk(*signalCliConfig, func(path string, info os.FileInfo, err error) error {
				filename := filepath.Base(path)
				if strings.HasPrefix(filename, "+") && info.Mode().IsRegular() {
					log.Debug("AUTO_RECEIVE_SCHEDULE: Calling receive for number ", filename)
					resp, err := http.Get("http://127.0.0.1:" + port + "/v1/receive/"+filename)
					if err != nil {
						log.Error("AUTO_RECEIVE_SCHEDULE: Couldn't call receive for number ", filename, ": ", err.Error())
					}
					if resp.StatusCode != 200 {
						jsonResp, err := ioutil.ReadAll(resp.Body)
						resp.Body.Close()
						if err != nil {
							log.Error("AUTO_RECEIVE_SCHEDULE: Couldn't read json response: ", err.Error())
							return nil
						}

						type ReceiveResponse struct {
							Error  string  `json:"error"`
						}
						var receiveResponse ReceiveResponse
						err = json.Unmarshal(jsonResp, &receiveResponse)
						if err != nil {
							log.Error("AUTO_RECEIVE_SCHEDULE: Couldn't parse json response: ", err.Error())
							return nil
						}

						log.Error("AUTO_RECEIVE_SCHEDULE: Couldn't call receive for number ", filename, ": ", receiveResponse)

					}
				}

				return nil
			})
			if err != nil {
				log.Fatal("AUTO_RECEIVE_SCHEDULE: Couldn't get registered numbers")
			}
		}))
		c.Start()
	}


	router.Run()
}


