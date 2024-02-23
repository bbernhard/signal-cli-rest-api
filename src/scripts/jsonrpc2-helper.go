package main

import (
	"fmt"
	"github.com/bbernhard/signal-cli-rest-api/utils"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

const supervisorctlConfigTemplate = `
[program:%s]
process_name=%s
command=bash -c "nc -l -p %d <%s | signal-cli --output=json --config %s jsonRpc >%s"
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
	fifoPathname := "/tmp/sigsocket1"

	jsonRpc2ClientConfig.AddEntry(utils.MULTI_ACCOUNT_NUMBER, utils.JsonRpc2ClientConfigEntry{TcpPort: tcpPort, FifoPathname: fifoPathname})

	os.Remove(fifoPathname) //remove any existing named pipe

	_, err := exec.Command("mkfifo", fifoPathname).Output()
	if err != nil {
		log.Fatal("Couldn't create fifo with name ", fifoPathname, ": ", err.Error())
	}

	uid := utils.GetEnv("SIGNAL_CLI_UID", "1000")
	gid := utils.GetEnv("SIGNAL_CLI_GID", "1000")
	_, err = exec.Command("chown", uid+":"+gid, fifoPathname).Output()
	if err != nil {
		log.Fatal("Couldn't change permissions of fifo with name ", fifoPathname, ": ", err.Error())
	}

	supervisorctlProgramName := "signal-cli-json-rpc-1"
	supervisorctlLogFolder := "/var/log/" + supervisorctlProgramName
	_, err = exec.Command("mkdir", "-p", supervisorctlLogFolder).Output()
	if err != nil {
		log.Fatal("Couldn't create log folder ", supervisorctlLogFolder, ": ", err.Error())
	}

	log.Info("Updated jsonrpc2.yml")

	//write supervisorctl config
	supervisorctlConfigFilename := "/etc/supervisor/conf.d/" + "signal-cli-json-rpc-1.conf"


	supervisorctlConfig := fmt.Sprintf(supervisorctlConfigTemplate, supervisorctlProgramName, supervisorctlProgramName,
		tcpPort, fifoPathname, signalCliConfigDir, fifoPathname, supervisorctlProgramName, supervisorctlProgramName)
	

	err = ioutil.WriteFile(supervisorctlConfigFilename, []byte(supervisorctlConfig), 0644)
	if err != nil {
		log.Fatal("Couldn't write ", supervisorctlConfigFilename, ": ", err.Error())
	}

	// write jsonrpc.yml config file
	err = jsonRpc2ClientConfig.Persist(signalCliConfigDir + "jsonrpc2.yml")
	if err != nil {
		log.Fatal("Couldn't persist jsonrpc2.yaml: ", err.Error())
	}
}
