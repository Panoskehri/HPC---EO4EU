package main

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

func UncompressInputsCmd(inputs []string) string {
	cmd := ""
	for _, input := range inputs {
		if strings.HasSuffix(input, ".zip") {
			cmd += fmt.Sprintf("unzip %s;", input)
		} else if strings.HasSuffix(input, "tar.gz") {
			cmd += fmt.Sprintf("tar -zxvf %s;", input)
		}
	}
	return cmd
}

func CompressOutputsCmd(outputs []string, file string) string {
	cmd := fmt.Sprintf("tar -czvf %s ", file)
	for _, output := range outputs {
		cmd += fmt.Sprintf("%s ", output)
	}
	return cmd
}

func ExecuteCmd(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	output, err := session.CombinedOutput(cmd)
	return string(output), err
}

func HasErrorOutput(client *ssh.Client, file string) bool { // return error also
	cmd := fmt.Sprintf("{ [ ! -e %s ] || [ -s %s ]; } && echo 'error' || echo 'no_error'", file, file)
	output, _ := ExecuteCmd(client, cmd)
	return strings.TrimSpace(output) == "error"
}

func ExistFile(client *ssh.Client, file string) (bool, error) {
	cmd := fmt.Sprintf("[ -e %s ] && echo 'exist' || echo 'not_exist'", file)
	output, err := ExecuteCmd(client, cmd)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) == "exist", nil
}
