package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment")
	}

	// Load SSH credentials
	user := os.Getenv("SLURM_USER")
	host := os.Getenv("SLURM_HOST")
	key := os.Getenv("SLURM_KEY")

	if user == "" || key == "" || host == "" {
		log.Fatal("SSH credentials are missing!")
	}

	// Load file holding workflow schema
	schemaPath := os.Getenv("WORKFLOW_SCHEMA")
	file, err := os.ReadFile(schemaPath)
	if err != nil {
		log.Fatalf("error reading schema file %s: %v", schemaPath, err)
	}

	// Load workflow schema
	var schema Schema
	if err := json.Unmarshal(file, &schema); err != nil {
		log.Fatalf("error parsing json: %v", err)
	}

	// Local file paths
	localBatchScripts, err := GetBatchScripts(schema, "templates/submit_tmpl.sh", "templates/step_tmpl.sh")
	if err != nil {
		log.Fatalf("error getting batch scripts from workflow %s: %v", schema.WorkflowID, err)
	}
	// localImages := GetImages(schema)
	// localJobInputs := GetDatalocation(schema)
	localImages := []string{}
	localJobInputs := []string{}

	// Remote file paths (needed for SCP)
	workdir := filepath.Join(schema.Workdir, schema.WorkflowID)
	remoteBatchScripts := GetRemotePaths(workdir, localBatchScripts)
	remoteImages := GetRemotePaths(workdir, localImages)
	remoteJobInputs := GetRemotePaths(workdir, localJobInputs)

	// Remote file paths (needed when the submitted job(s) complete or fail)
	jobStatus := filepath.Join(schema.WorkflowID, os.Getenv("JOB_STATUS"))
	remoteResults := filepath.Join(schema.Workdir, schema.WorkflowID, os.Getenv("RESULTS"))
	localResults := filepath.Join(schema.WorkflowID, os.Getenv("RESULTS"))
	// remoteOutputFiles := GetOutputs(schema)
	remoteOutputFiles := []string{}

	// Initialize SSH client
	sshClient, err := ConnectToSSHKey(user, key, host)
	if err != nil {
		log.Fatalf("Failed to connect to SSH client: %v", err)
	}
	defer sshClient.Close()

	// Initialize SCP client
	scpClient, err := scp.NewClientBySSH(sshClient)
	if err != nil {
		log.Fatalf("Error creating SCP client: %v", err)
	}

	// Create transfer pairs of local-remote files for SCP
	transfers := GetTransfers([][]string{localBatchScripts, localImages, localJobInputs}, [][]string{remoteBatchScripts, remoteImages, remoteJobInputs})

	// Transfer all local files to remote via SCP
	fmt.Println("Starting data transfer...")
	err = TransferData(scpClient, transfers)
	if err != nil {
		log.Fatalf("Error while transfering data: %v", err)
	}
	fmt.Println("Transfers complete!")

	// // Uncompress input data
	// cmd := fmt.Sprintf("cd %s;%s", workdir, UncompressInputsCmd(remoteJobInputs)) // cd to working directory first
	// output, err := ExecuteCmd(sshClient, cmd)
	// if err != nil {
	// 	log.Fatalf("Uncompression of input data failed: %v\nOutput: %s", err, output)
	// }
	// fmt.Println("Uncompressed all input data!")

	// Submit batch script to slurm
	batchScript := remoteBatchScripts[0] // always sbatch the first one
	cmd := fmt.Sprintf("sbatch %s", batchScript)
	output, err := ExecuteCmd(sshClient, cmd)
	if err != nil {
		log.Fatalf("sbatch execution failed: %v\nOutput: %s", err, output)
	}

	// Extracting job ID (assuming "Submitted batch job xxxx" format)
	sbatchResult := string(output)
	fields := strings.Fields(sbatchResult)
	jobID := "-1"
	if len(fields) > 0 {
		jobID = fields[len(fields)-1]
		fmt.Printf("Job successfully queued with ID: %s\n", jobID)
	}

	// Get job's state through polling
	fmt.Printf("Retrieving job %s state...\n", jobID)
	state, err := PollingJobCompletionV2(sshClient, jobID)
	if err != nil {
		log.Fatalf("Job %s polling failed: %s", jobID, err)
	} else if state != "COMPLETED" {
		err = SaveJobStatusesV2(sshClient, []string{jobID}, jobStatus)
		if err != nil {
			log.Fatalf("Failed to save status of jobs to %s: %s", jobStatus, err)
		}
		err = DownloadData(sshClient, scpClient, GetLogFiles(sshClient, workdir, []string{jobID}), remoteResults, localResults)
		if err != nil {
			log.Fatalf("Download of data failed: %s", err)
		}
		fmt.Println("Downloaded all data locally")
		log.Fatalf("Job %s terminated with state: %s", jobID, state)
	}

	// Get list of job ids that were submitted by the batch script
	logFile := filepath.Join(workdir, fmt.Sprintf("result_%s.log", jobID))
	ids, err := GetJobIDs(sshClient, logFile)
	ids = append([]string{jobID}, ids...) // include the submitted job also
	if err != nil {
		log.Fatalf("Failed to get list of job IDs submitted by job %s: %s", jobID, err)
	}

	// Get last job's state through polling (assuming chained dependency between all the jobs)
	jobID = ids[len(ids)-1]
	fmt.Printf("Retrieving job %s state...\n", jobID)
	state, err = PollingJobCompletionV2(sshClient, jobID)
	if err != nil {
		log.Fatalf("Job %s polling failed: %s", jobID, err)
	} else if state != "COMPLETED" {
		err = SaveJobStatusesV2(sshClient, ids, jobStatus)
		if err != nil {
			log.Fatalf("Failed to save status of jobs to %s: %s", jobStatus, err)
		}
		err = DownloadData(sshClient, scpClient, GetLogFiles(sshClient, workdir, ids), remoteResults, localResults)
		if err != nil {
			log.Fatalf("Download of data failed: %s", err)
		}
		fmt.Println("Downloaded all data locally")
		log.Fatalf("Job %s terminated with state: %s", jobID, state)
	}

	// Downloading all output data locally
	remoteOutputFiles = append(remoteOutputFiles, GetLogFiles(sshClient, workdir, ids)...)
	err = SaveJobStatusesV2(sshClient, ids, jobStatus)
	if err != nil {
		log.Fatalf("Failed to save status of jobs to %s: %s", jobStatus, err)
	}
	err = DownloadData(sshClient, scpClient, remoteOutputFiles, remoteResults, localResults)
	if err != nil {
		log.Fatalf("Download of data failed: %s", err)
	}
	fmt.Println("Downloaded all data locally")
}
