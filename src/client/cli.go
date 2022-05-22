package client

import (
	"strings"
	"errors"
	"os/exec"
	"bytes"
	"time"
	"bufio"
	log "github.com/sirupsen/logrus"
	utils "github.com/bbernhard/signal-cli-rest-api/utils"
)

type CliClient struct {
	signalCliMode            SignalCliMode
	signalCliApiConfig       *utils.SignalCliApiConfig
}

func NewCliClient(signalCliMode SignalCliMode, signalCliApiConfig *utils.SignalCliApiConfig) *CliClient {
	return &CliClient {
		signalCliMode: signalCliMode,
		signalCliApiConfig: signalCliApiConfig,
	}
}

func (s *CliClient) Execute(wait bool, args []string, stdin string) (string, error) {
	containerId, err := getContainerId()

	log.Debug("If you want to run this command manually, run the following steps on your host system:")
	if err == nil {
		log.Debug("*) docker exec -it ", containerId, " /bin/bash")
	} else {
		log.Debug("*) docker exec -it <container id> /bin/bash")
	}

	signalCliBinary := ""
	if s.signalCliMode == Normal {
		signalCliBinary = "signal-cli"
	} else if s.signalCliMode == Native {
		signalCliBinary = "signal-cli-native"
	} else {
		return "", errors.New("Invalid signal-cli mode")
	}

	//check if args contain number
	trustModeStr := ""
	for i, arg := range args {
		if (arg == "-a" || arg == "--account") && (((i+1) < len(args)) && (utils.IsPhoneNumber(args[i+1]))) {
			number := args[i+1]
			trustMode, err := s.signalCliApiConfig.GetTrustModeForNumber(number)
			if err == nil {
				trustModeStr, err = utils.TrustModeToString(trustMode)
				if err != nil {
					trustModeStr = ""
					log.Error("Invalid trust mode: ", trustModeStr)
				}
			}
			break
		}
	}

	if trustModeStr != "" {
		args = append([]string{"--trust-new-identities", trustModeStr}, args...)
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

