package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
)

func main() {
	// 1. Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment")
	}

	user := os.Getenv("SLURM_USER")
	pass := os.Getenv("SLURM_PASS")
	host := os.Getenv("SLURM_HOST")

	// Local file paths from your .env
	localSlurmReq := os.Getenv("SLURM_SCRIPT") // e.g., job.sh
	localJobScript := os.Getenv("JOB_SCRIPT")  // e.g., job_script.py
	localJobInput := os.Getenv("JOB_INPUT")    // e.g., data.csv

	// Remote paths , needed for scp, subject to change
	remotebatchscript := "/home/" + user + "/job.sbatch"
	remoteJobScript := "/home/" + user + "/job_script.py"
	remoteJobInput := "/home/" + user + "/job_input.txt"

	if user == "" || pass == "" || host == "" {
		log.Fatal("Essential environment variables are missing!")
	}

	// 2. SSH Configuration
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)}, // subject to change
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// 3. Connect
	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	// 4. Initialize SCP
	scpClient, err := scp.NewClientBySSH(client)
	if err != nil {
		log.Fatalf("Error creating SCP client: %v", err)
	}

	// Define a slice of transfers to handle them all at once
	type transfer struct {
		local  string
		remote string
	}

	// Remote paths , needed for scp
	// Subject to change based on HPC filesystem structure and user home directory(and future project logic)
	transfers := []transfer{
		{localSlurmReq, remotebatchscript},
		{localJobScript, remoteJobScript},
		{localJobInput, remoteJobInput},
	}

	// Print out the planned transfers for verification
	for _, t := range transfers {
		fmt.Printf("Local: %s, Remote: %s\n", t.local, t.remote)
	}

	fmt.Println("Starting full file transfer...")

	for _, t := range transfers {
		if t.local == "" {
			continue // Skip if the env variable wasn't set
		}

		err := func() error {
			f, err := os.Open(t.local)
			if err != nil {
				return err
			}
			defer f.Close()

			return scpClient.CopyFile(context.Background(), f, t.remote, "0755")
		}()

		if err != nil {
			log.Fatalf("Failed to upload %s: %v", t.local, err)
		}
		fmt.Printf("Successfully uploaded %s to %s\n", t.local, t.remote)
	}

	fmt.Println("All transfers complete!")

	// New session for command execution
	session, err := client.NewSession()
	if err != nil {
		log.Fatalf("Failed to create session for sbatch: %v", err)
	}
	defer session.Close()

	remoteSbatchPath := "/home/" + user + "/job.sbatch"
	cmd := fmt.Sprintf("sbatch %s", remoteSbatchPath)

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		log.Fatalf("sbatch execution failed: %v\nOutput: %s", err, string(output))
	}

	// Example output: "Submitted batch job 12345"
	sbatchResult := string(output)
	fmt.Printf("Slurm Output: %s", sbatchResult)

	// Extracting JobID for later polling (Assuming: "Submitted batch job 12345")
	fields := strings.Fields(sbatchResult)
	jobID := "-1"
	if len(fields) > 0 {
		jobID = fields[len(fields)-1]
		fmt.Printf("Job successfully queued with ID: %s\n", jobID)
	}

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
