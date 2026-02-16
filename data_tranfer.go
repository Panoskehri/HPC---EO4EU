package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
)

func go_scp() {
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
	localJobData := os.Getenv("JOB_DATA")      // e.g., data.csv

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
		{localSlurmReq, "/home/" + user + "/job.sbatch"},
		{localJobScript, "/home/" + user + "/job_script.py"},
		{localJobData, "/home/" + user + "/data.csv"},
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

}
