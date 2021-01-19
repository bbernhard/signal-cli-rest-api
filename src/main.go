package main

import (
	"flag"
	"os"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/bbernhard/signal-cli-rest-api/api"
	_ "github.com/bbernhard/signal-cli-rest-api/docs"

)



// @title Signal Cli REST API
// @version 1.0
// @description This is the Signal Cli REST API documentation.

// @tag.name General
// @tag.description List general information.

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

	api := api.NewApi(*signalCliConfig, *attachmentTmpDir, *avatarTmpDir)
	v1 := router.Group("/v1")
	{
		about := v1.Group("/about")
		{
			about.GET("", api.About)
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
	}

	v2 := router.Group("/v2")
	{
		sendV2 := v2.Group("/send")
		{
			sendV2.POST("", api.SendV2)
		}
	}

	swaggerPort := getEnv("PORT", "8080")

	swaggerUrl := ginSwagger.URL("http://127.0.0.1:" + string(swaggerPort) + "/swagger/doc.json")
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, swaggerUrl))

	router.Run()
}

func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
