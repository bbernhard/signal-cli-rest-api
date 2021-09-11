package main

import (
	"flag"
	"strings"
	"strconv"
	"os"
	"fmt"
	"os/exec"
	"io/ioutil"
	"path/filepath"
	"github.com/bbernhard/signal-cli-rest-api/utils"
	log "github.com/sirupsen/logrus"
)

const supervisorctlConfigTemplate = `
[program:%s]
environment=JAVA_HOME=/opt/java/openjdk
process_name=%s
command=bash -c "nc -l -p %d <%s | signal-cli --output=json -u %s --config /home/.local/share/signal-cli/ jsonRpc >%s"
autostart=true
autorestart=true
startretries=10
user=signal-api
directory=/usr/bin/
redirect_stderr=true
stdout_logfile=/var/log/%s/out.log
stderr_logfile=/var/log/%s/err.log
stdout_logfile_maxbytes=50MB
stdout_logfile_backups=10
numprocs=1
`

func main() {
	signalCliConfigDir := flag.String("signal-cli-config-dir", "/home/.local/share/signal-cli/", "Path to signal-cli config directory")

	jsonRpc2ClientConfig := utils.NewJsonRpc2ClientConfig()

	var tcpBasePort int64 = 6000
	fifoBasePathName := "/tmp/sigsocket"
	var ctr int64 = 0

	err := filepath.Walk(*signalCliConfigDir, func(path string, info os.FileInfo, err error) error {
		filename := filepath.Base(path)
		if strings.HasPrefix(filename, "+") && info.Mode().IsRegular() {
			fifoPathname := fifoBasePathName + strconv.FormatInt(ctr, 10)
			tcpPort := tcpBasePort + ctr
			jsonRpc2ClientConfig.AddEntry(filename, utils.ConfigEntry{TcpPort: tcpPort,  FifoPathname: fifoPathname})
			ctr += 1
			_, err = exec.Command("mkfifo", fifoPathname).Output()
			if err != nil {
				log.Fatal("Couldn't create fifo with name ", fifoPathname, ": ", err.Error())
			}

			_, err = exec.Command("chown", "1000:1000", fifoPathname).Output()
			if err != nil {
				log.Fatal("Couldn't change permissions of fifo with name ", fifoPathname, ": ", err.Error())
			}

			supervisorctlProgramName := "signal-cli-json-rpc-" + strconv.FormatInt(ctr, 10)
			supervisorctlLogFolder := "/var/log/" + supervisorctlProgramName
			_, err = exec.Command("mkdir", "-p", supervisorctlLogFolder).Output()
			if err != nil {
				log.Fatal("Couldn't create log folder ", supervisorctlLogFolder, ": ", err.Error())
			}

			//write supervisorctl config
			supervisorctlConfigFilename := "/etc/supervisor/conf.d/"+"signal-cli-json-rpc-" + strconv.FormatInt(ctr, 10) + ".conf"
			supervisorctlConfig := fmt.Sprintf(supervisorctlConfigTemplate, supervisorctlProgramName, supervisorctlProgramName,
												tcpPort, fifoPathname, filename, fifoPathname, supervisorctlProgramName, supervisorctlProgramName)
			log.Info(supervisorctlConfig)
			err = ioutil.WriteFile(supervisorctlConfigFilename, []byte(supervisorctlConfig), 0644)
			if err != nil {
				log.Fatal("Couldn't write ", supervisorctlConfigFilename, ": ", err.Error())
			}
		}
		return nil
	})

	// write jsonrpc.yml config file
	err = jsonRpc2ClientConfig.Persist(*signalCliConfigDir + "jsonrpc2.yml")
	if err != nil {
		log.Fatal("Couldn't persist jsonrpc2.yaml: ", err.Error())
	}
}
