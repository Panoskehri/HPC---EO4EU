package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

func GetPendingReason(info string) string {
	re := regexp.MustCompile(`Reason=([^\s]+)`)
	match := re.FindStringSubmatch(info)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func PollingJobCompletion(client *ssh.Client, id string) (string, error) {
	interval, err := strconv.Atoi(PollingInterval)
	if err != nil {
		interval = 900 // default if conversion fails
	}
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
			return "PENDING (DependencyNeverSatisfied)", nil
		} else if state != "PENDING" && state != "PREEMPTED" && state != "RUNNING" && state != "SUSPENDED" {
			return state, nil
		}
		time.Sleep(time.Duration(interval) * time.Second)
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
