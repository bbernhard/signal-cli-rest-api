package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/satori/go.uuid"
	"github.com/gin-gonic/gin"
	"github.com/h2non/filetype"
	"os/exec"
	"time"
	"errors"
	"flag"
	"bytes"
	"os"
	"encoding/base64"
	//"strings"
)

func runSignalCli(args []string) error {
	cmd := exec.Command("signal-cli", args...)
	var errBuffer bytes.Buffer
	cmd.Stderr = &errBuffer

	err := cmd.Start()
	if err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-time.After(60 * time.Second):
		err := cmd.Process.Kill()
		if err != nil {
			return err
		}
		return errors.New("process killed as timeout reached")
	case err := <-done:
		if err != nil {
			return errors.New(errBuffer.String())
		}
	}
	return nil
}

func main() {
	signalCliConfig := flag.String("signal-cli-config", "/home/.local/share/signal-cli/", "Config directory where signal-cli config is stored")
	attachmentTmpDir := flag.String("attachment-tmp-dir", "/tmp/", "Attachment tmp directory")
	flag.Parse()

	router := gin.Default()
	log.Info("Started Signal Messenger REST API")

	router.POST("/v1/register/:number", func(c *gin.Context) {
		number := c.Param("number")

		if number == "" {
			c.JSON(400, "Please provide a number")
			return
		}

		err := runSignalCli([]string{"--config", *signalCliConfig, "-u", number, "register"})
		if err != nil {
			c.JSON(400, err.Error())
			return
		}
		c.JSON(201, nil)
	})

	router.POST("/v1/register/:number/verify/:token", func(c *gin.Context) {
		number := c.Param("number")
		token := c.Param("token")

		if number == "" {
			c.JSON(400, "Please provide a number")
			return
		}

		if token == "" {
			c.JSON(400, gin.H{"error": "Please provide a verification code"})
			return
		}

		
		err := runSignalCli([]string{"--config", *signalCliConfig, "-u", number, "verify", token})
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, nil)
	})

	router.POST("/v1/send", func(c *gin.Context) {
		type Request struct{
			Number string `json:"number"`
			Recipients []string `json:"recipients"`
			Message string `json:"message"`
			Base64Attachment string `json:"base64_attachment"`
		}
		var req Request
		err := c.BindJSON(&req)
		if err != nil {
			c.JSON(400, "Couldn't process request - invalid request")
			return
		}

		cmd := []string{"--config", *signalCliConfig, "-u", req.Number, "send", "-m", req.Message}
		cmd = append(cmd, req.Recipients...)
	
		attachmentTmpPath := ""
		if(req.Base64Attachment != "") {
			u, err := uuid.NewV4()
			if err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
                return
			}
			
			dec, err := base64.StdEncoding.DecodeString(req.Base64Attachment)
    		if err != nil {
        		c.JSON(400, gin.H{"error": err.Error()})
    			return
			}

			fType, err := filetype.Get(dec)
			if err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}

			attachmentTmpPath := *attachmentTmpDir + u.String() + "." + fType.Extension

    		f, err := os.Create(attachmentTmpPath)
    		if err != nil {
        		c.JSON(400, gin.H{"error": err.Error()})
                return
    		}
    		defer f.Close()

    		if _, err := f.Write(dec); err != nil {
        		c.JSON(400, gin.H{"error": err.Error()})
                return
    		}
    		if err := f.Sync(); err != nil {
        		c.JSON(400, gin.H{"error": err.Error()})
                return
    		}

			cmd = append(cmd, "-a")
			cmd = append(cmd , attachmentTmpPath)
		}
		
		err = runSignalCli(cmd)
		if err != nil {
			if attachmentTmpPath != "" {
				os.Remove(attachmentTmpPath)
			}
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, nil)
	})

	router.Run()
}
