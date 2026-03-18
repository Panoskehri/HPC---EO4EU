package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
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

func GetPendingReason(info string) string {
	re := regexp.MustCompile(`Reason=([^\s]+)`)
	match := re.FindStringSubmatch(info)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func PollingJobCompletion(client *ssh.Client, id string) (string, error) {
	state := ""
	for true {
		cmd := fmt.Sprintf("scontrol show job %s", id)
		output, err := ExecuteCmd(client, cmd)
		if err != nil {
			return state, fmt.Errorf("scontrol execution for job %s failed: %v\nOutput: %s", id, err, output)
		}

		re := regexp.MustCompile(`JobState=([^\s]+)`)
		match := re.FindStringSubmatch(output)
		if len(match) > 1 {
			state = match[1]
			fmt.Println("Job State:", state)
		} else {
			return state, fmt.Errorf("job %s state not found", id)
		}

		if state == "PENDING" && GetPendingReason(output) == "DependencyNeverSatisfied" { // when afterok dependency is used (job has to be cancelled)
			return fmt.Sprint("PENDING (DependencyNeverSatisfied)"), nil
		} else if state != "PENDING" && state != "PREEMPTED" && state != "RUNNING" && state != "SUSPENDED" {
			return state, nil
		} else {
			time.Sleep(2 * time.Second)
		}
	}
	return state, nil
}

func GetJobIDs(client *ssh.Client, file string) ([]string, error) {
	cmd := fmt.Sprintf("cat %s", file)
	output, err := ExecuteCmd(client, cmd)
	if err != nil {
		return nil, fmt.Errorf("cat file %s execution failed: %v\nOutput: %s", file, err, output)
	}
	re := regexp.MustCompile(`Job ID:\s*(\d+)`)
	matches := re.FindAllStringSubmatch(output, -1)
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			ids = append(ids, match[1])
		}
	}
	return ids, nil
}

func GetLogFiles(client *ssh.Client, workdir string, ids []string) []string {
	logFiles := make([]string, 0, 2*len(ids))
	for _, id := range ids {
		logFile := filepath.Join(workdir, fmt.Sprintf("result_%s.log", id))
		errFile := filepath.Join(workdir, fmt.Sprintf("result_%s.err", id))
		logFiles = append(logFiles, logFile)
		logFiles = append(logFiles, errFile)
	}
	return logFiles
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

func ExtractJobDetails(client *ssh.Client, id string) (string, error) {
	var firstLine, stateLine string
	cmd := fmt.Sprintf("scontrol show job %s", id)
	output, err := ExecuteCmd(client, cmd)
	if err != nil {
		return "", fmt.Errorf("scontrol execution for job %s failed: %v\nOutput: %s", id, err, output)
	}
	scanner := bufio.NewScanner(strings.NewReader(output))
	isFirst := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if isFirst {
			firstLine = line
			isFirst = false
			continue
		}
		if strings.HasPrefix(line, "JobState=") {
			stateLine = line
			break
		}
	}
	return fmt.Sprintf("%s %s", firstLine, stateLine), nil
}

func SaveJobStatuses(client *ssh.Client, ids []string, file string) error {
	statuses := ""
	for _, id := range ids {
		status, err := ExtractJobDetails(client, id)
		if err != nil {
			return fmt.Errorf("Could not save job statuses: %s", err)
		}
		statuses += status + "\n"
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("Could not save job statuses: %s", err)
	}
	defer f.Close()
	if _, err := f.WriteString(statuses); err != nil {
		return fmt.Errorf("Could not save job statuses: %s", err)
	}
	return nil
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment")
	}

	// Load SSH credentials
	user := os.Getenv("SLURM_USER")
	pass := os.Getenv("SLURM_PASS")
	host := os.Getenv("SLURM_HOST")
	key := os.Getenv("SLURM_KEY")

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

	// Remote file paths (needed when the submitted job(s) complete or fail)
	jobStatus := os.Getenv("JOB_STATUS")
	results_file := os.Getenv("RESULTS")
	remoteOutputFiles := strings.Split(os.Getenv("JOB_OUTPUTS"), ":")

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
	transfers := GetTransfers([][]string{localBatchScripts, localJobScripts, localJobInputs}, [][]string{remoteBatchScripts, remoteJobScripts, remoteJobInputs})

	// Transfer all local files to remote via SCP
	fmt.Println("Starting data transfer...")
	err = TransferData(scpClient, transfers)
	if err != nil {
		log.Fatalf("Error while transfering data: %v", err)
	}
	fmt.Println("Transfers complete!")

	// Uncompress input data
	cmd := fmt.Sprintf("cd %s;%s", workdir, UncompressInputsCmd(remoteJobInputs)) // cd to working directory first
	output, err := ExecuteCmd(sshClient, cmd)
	if err != nil {
		log.Fatalf("Uncompression of input data failed: %v\nOutput: %s", err, output)
	}
	fmt.Println("Uncompressed all input data!")

	// Submit batch script to slurm
	batchScript := remoteBatchScripts[0] // always sbatch the first one
	cmd = fmt.Sprintf("sbatch %s", batchScript)
	output, err = ExecuteCmd(sshClient, cmd)
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
	state, err := PollingJobCompletion(sshClient, jobID)
	if err != nil {
		log.Fatalf("Job %s polling failed: %s", jobID, err)
	} else if state != "COMPLETED" {
		err = SaveJobStatuses(sshClient, []string{jobID}, jobStatus)
		if err != nil {
			log.Fatalf("Failed to save status of jobs to %s: %s", jobStatus, err)
		}
		err = DownloadData(sshClient, scpClient, GetLogFiles(sshClient, workdir, []string{jobID}), results_file)
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
	state, err = PollingJobCompletion(sshClient, jobID)
	if err != nil {
		log.Fatalf("Job %s polling failed: %s", jobID, err)
	} else if state != "COMPLETED" {
		err = SaveJobStatuses(sshClient, ids, jobStatus)
		if err != nil {
			log.Fatalf("Failed to save status of jobs to %s: %s", jobStatus, err)
		}
		err = DownloadData(sshClient, scpClient, GetLogFiles(sshClient, workdir, ids), results_file)
		if err != nil {
			log.Fatalf("Download of data failed: %s", err)
		}
		fmt.Println("Downloaded all data locally")
		log.Fatalf("Job %s terminated with state: %s", jobID, state)
	}

	// Downloading all output data locally
	remoteOutputFiles = append(remoteOutputFiles, GetLogFiles(sshClient, workdir, ids)...)
	err = SaveJobStatuses(sshClient, ids, jobStatus)
	if err != nil {
		log.Fatalf("Failed to save status of jobs to %s: %s", jobStatus, err)
	}
	err = DownloadData(sshClient, scpClient, remoteOutputFiles, results_file)
	if err != nil {
		log.Fatalf("Download of data failed: %s", err)
	}
	fmt.Println("Downloaded all data locally")
}
