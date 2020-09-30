package system

import (
	"os/exec"
	"errors"
	"bytes"
	"time"
	"io/ioutil"
	"strings"
	"bufio"
	"os"
)

func UseDbus() bool {
	useDbus := os.Getenv("USE_DBUS")
	if useDbus == "true" {
		return true
	}

	return false
}

func CleanupTmpFiles(paths []string) {
	for _, path := range paths {
		os.Remove(path)
	}
}

func RunCommand(command string, args []string) (string, error) {
	cmd := exec.Command(command, args...)
	
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

func RunSignalCli(wait bool, args []string) (string, error) {
	cmd := exec.Command("signal-cli", args...)
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


func getRegisteredNumbers(path string) ([]string, error) {
	registeredNumbers := []string{}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return registeredNumbers, err
	}

	for _, f := range files {
		if !f.IsDir() {
			registeredNumbers = append(registeredNumbers, f.Name())
		}
	}

	return registeredNumbers, nil
}

func StartSystemDbus() error {
	_, err := RunCommand("dbus-daemon", []string{"--system"})
	return err
}

func StartSignalCliDbusDaemon(signalCliConfig string) error {
	registeredNumbers, err := getRegisteredNumbers(signalCliConfig+"/data/")
	if err != nil {
		return err
	}

	if len(registeredNumbers) > 1 {
		return errors.New("DBUS only supports max. one number!")
	}

	if len(registeredNumbers) != 1 {
		return errors.New("No registered number found!")
	}

	supervisorConfPath := "/etc/supervisor/conf.d/signal-cli.conf"

	data, err := ioutil.ReadFile(supervisorConfPath)
    if err != nil {
		return err
	}

	out := strings.Replace(string(data), "!#MY_NUMBER#!", registeredNumbers[0], -1)
	
	err = ioutil.WriteFile(supervisorConfPath, []byte(out), 0644)
	if err != nil {
		return err
	}

	_, err = RunCommand("service", []string{"supervisor", "start"})
	return err
}

/*func SendMessage() error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	var s string
	err = conn.Object("org.asamk.Signal", "/").Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&s)
	if err != nil {
		return err
	}
}*/
