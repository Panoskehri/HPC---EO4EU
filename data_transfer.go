package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/bramvdbogaerde/go-scp"
	"golang.org/x/crypto/ssh"
)

type Transfer struct {
	Local  string
	Remote string
}

type Pair struct {
	LocalFiles  []string
	RemoteFiles []string
}

func CreatePairs(local, remote [][]string) []Pair {
	pairs := make([]Pair, len(local))
	for i := range local {
		pairs[i] = Pair{LocalFiles: local[i], RemoteFiles: remote[i]}
	}
	return pairs
}

func GetRemotePaths(dir string, files []string) []string {
	remoteFiles := make([]string, len(files))
	for i := range files {
		remoteFiles[i] = filepath.Join(dir, filepath.Base(files[i]))
	}
	return remoteFiles
}

func GetTransfers(localFiles [][]string, remoteFiles [][]string) []Transfer {
	var transfers []Transfer
	pairs := CreatePairs(localFiles, remoteFiles)

	for _, pair := range pairs {
		for i := range pair.LocalFiles {
			transfers = append(transfers, Transfer{pair.LocalFiles[i], pair.RemoteFiles[i]})
		}
	}

	return transfers
}

func CopyFileSCP(client scp.Client, local string, remote string) error {
	f, err := os.Open(local)
	if err != nil {
		return err
	}
	defer f.Close()
	return client.CopyFile(context.Background(), f, remote, "0755")
}

func TransferData(client scp.Client, transfers []Transfer) error {
	for _, t := range transfers {
		if t.Local == "" {
			continue
		}
		err := CopyFileSCP(client, t.Local, t.Remote)
		if err != nil {
			log.Fatalf("Failed to upload %s: %v", t.Local, err)
			return err
		} else {
			fmt.Printf("Successfully uploaded %s to %s\n", t.Local, t.Remote)
		}
	}
	return nil
}

func DownloadData(sshClient *ssh.Client, scpClient scp.Client, remoteOutputFiles []string, results_file string) error {
	// Keep only files and directories that exist
	existingOutputFiles := make([]string, 0, len(remoteOutputFiles))
	for _, file := range remoteOutputFiles {
		exist, err := ExistFile(sshClient, file)
		if err != nil {
			return err
		}
		if exist {
			existingOutputFiles = append(existingOutputFiles, file)
		}
	}

	// Compress output data
	cmd := CompressOutputsCmd(existingOutputFiles, results_file)
	output, err := ExecuteCmd(sshClient, cmd)
	if err != nil {
		return fmt.Errorf("Compression of output data failed: %v\nOutput: %s", err, output)
	}
	fmt.Printf("Compressed all output data to %s\n", results_file)

	// Donwload output data locally
	fmt.Printf("Attempting to download output data from: %s\n", results_file)
	destFile, err := os.Create(filepath.Base(results_file))
	if err != nil {
		return fmt.Errorf("Failed to create local output file: %v", err)
	}
	defer destFile.Close()
	err = scpClient.CopyFromRemote(context.Background(), destFile, results_file)
	if err != nil {
		return fmt.Errorf("Could not download output data: %v\n", err)
	} else {
		fmt.Printf("Successfully downloaded job output to: %s\n", destFile.Name())
	}
	return nil
}
