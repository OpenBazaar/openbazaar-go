package cmd

import (
	"os/exec"
	"runtime"
)

func Execute(args []string) error {

	var command []string
	switch runtime.GOOS {
	case "darwin":
		command = []string{"open"}
	case "windows":
		command = []string{"cmd"}
	default:
		command = []string{`sh`, `-c`}
	}
	cmd := exec.Command(command[0], append(command[1:], `openssl req -newkey rsa:2048 -sha256 -nodes -keyout key.pem -x509 -days 365 -out cert.pem -subj "/C=. /ST=. /L=. /O=. Company/CN=."`)...)
	cmd.Start()

	return nil

}
