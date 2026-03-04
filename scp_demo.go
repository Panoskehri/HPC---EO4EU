package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/joho/godotenv"
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

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment")
	}

	// Load SSH credentials
	user := os.Getenv("SLURM_USER")
	pass := os.Getenv("SLURM_PASS")
	host := os.Getenv("SLURM_HOST")

	if user == "" || pass == "" || host == "" {
		log.Fatal("SSH credentials are missing!")
	}

	// Local file paths
	localBatchScripts := strings.Split(os.Getenv("BATCH_SCRIPTS"), ":")
	localJobScripts := strings.Split(os.Getenv("JOB_SCRIPTS"), ":")
	localJobInputs := strings.Split(os.Getenv("JOB_INPUTS"), ":")

	// Remote file paths (needed for SCP)
	workdir := os.Getenv("SLURM_WORKDIR")
	remoteBatchScripts := GetRemotePaths(workdir, localBatchScripts)
	remoteJobScripts := GetRemotePaths(workdir, localJobScripts)
	remoteJobInputs := GetRemotePaths(workdir, localJobInputs)

	// SSH Configuration
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)}, // subject to change
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// Connect to SSH
	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	// Initialize SCP
	scpClient, err := scp.NewClientBySSH(client)
	if err != nil {
		log.Fatalf("Error creating SCP client: %v", err)
	}

	// Create transfer pairs of local-remote files for SCP
	transfers := GetTransfers([][]string{localBatchScripts, localJobScripts, localJobInputs}, [][]string{remoteBatchScripts, remoteJobScripts, remoteJobInputs})

	fmt.Println("Starting full file transfer...")

	// Copy each local file to remote via SCP
	for _, t := range transfers {
		if t.Local == "" {
			continue
		}
		err := CopyFileSCP(scpClient, t.Local, t.Remote)
		if err != nil {
			log.Fatalf("Failed to upload %s: %v", t.Local, err)
		} else {
			fmt.Printf("Successfully uploaded %s to %s\n", t.Local, t.Remote)
		}
	}

	fmt.Println("Transfers complete!")

	// TODO: (1) unzip dataset, (2) κράτα το πρώτο slurm script και να κάνεις sbatch αυτό

	// // New session for command execution
	// session, err := client.NewSession()
	// if err != nil {
	// 	log.Fatalf("Failed to create session for sbatch: %v", err)
	// }
	// defer session.Close()

	// remoteSbatchPath := workdir + "/job.sbatch"
	// cmd := fmt.Sprintf("sbatch %s", remoteSbatchPath)

	// output, err := session.CombinedOutput(cmd)
	// if err != nil {
	// 	log.Fatalf("sbatch execution failed: %v\nOutput: %s", err, string(output))
	// }

	// // Example output: "Submitted batch job 12345"
	// sbatchResult := string(output)
	// fmt.Printf("Slurm Output: %s", sbatchResult)

	// // Extracting JobID for later polling (Assuming: "Submitted batch job 12345")
	// fields := strings.Fields(sbatchResult)
	// jobID := "-1"
	// if len(fields) > 0 {
	// 	jobID = fields[len(fields)-1]
	// 	fmt.Printf("Job successfully queued with ID: %s\n", jobID)
	// }

	// JOB SUBMISSION COMPLETE
	// OUTPUT RETRIEVAL LOGIC, to be completed

	// fmt.Println("Waiting 15 seconds for job processing...")
	// time.Sleep(120 * time.Second)

	// remoteOutputFile := fmt.Sprintf("/home/%s/job-%s.out", user, jobID)
	// localOutputFile := fmt.Sprintf("job_output_%s.log", jobID)

	// fmt.Printf("Attempting to download output from: %s\n", remoteOutputFile)

	// destFile, err := os.Create(localOutputFile)
	// if err != nil {
	// 	log.Fatalf("Failed to create local output file: %v", err)
	// }
	// defer destFile.Close()

	// err = scpClient.CopyFromRemote(context.Background(), destFile, remoteOutputFile)
	// if err != nil {
	// 	fmt.Printf("Note: Could not download output yet (Job might still be pending): %v\n", err)
	// } else {
	// 	fmt.Printf("Successfully downloaded job output to: %s\n", localOutputFile)
	// }

}
