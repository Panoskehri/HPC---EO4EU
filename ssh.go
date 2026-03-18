package main

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

func ConnectToSSH(user string, pass string, host string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)}, // subject to change
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	client, err := ssh.Dial("tcp", host, config)
	return client, err
}

func GetKeyAuth(path string) (ssh.AuthMethod, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %v", err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %v", err)
	}
	return ssh.PublicKeys(signer), nil
}

func ConnectToSSHKey(user string, keyPath string, host string) (*ssh.Client, error) {
	authMethod, err := GetKeyAuth(keyPath)
	if err != nil {
		return nil, fmt.Errorf("ssh auth setup failed: %v", err)
	}
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // keep this for testing, but use ssh.CheckHostKey for production!
	}
	client, err := ssh.Dial("tcp", host, config)
	return client, err
}
