package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var PollingInterval = os.Getenv("POLLING_INTERVAL")
var InitialPollingInterval = 10

func GetPendingReasonV2(client *ssh.Client, id string) (string, error) {
	reason := ""
	cmd := fmt.Sprintf("squeue -j %s", id)
	output, err := ExecuteCmd(client, cmd)
	if err != nil {
		return reason, fmt.Errorf("squeue execution for job %s failed: %v\nOutput: %s", id, err, output)
	}
	re := regexp.MustCompile(`\((.*?)\)`)
	lines := strings.SplitSeq(strings.TrimSpace(output), "\n")
	for line := range lines {
		if strings.Contains(line, "JOBID") { // skip the header line
			continue
		}
		match := re.FindStringSubmatch(line)
		if len(match) > 1 {
			reason := match[1]
			return reason, nil
		}
	}
	return reason, nil
}

func PollingJobCompletionV2(client *ssh.Client, id string) (string, error) {
	interval, err := strconv.Atoi(PollingInterval)
	if err != nil {
		interval = 900 // default if conversion fails
	}
	state := ""
	time.Sleep(time.Duration(InitialPollingInterval) * time.Second) // needed to wait a little bit for sacct to return something
	for true {
		cmd := fmt.Sprintf("sacct -j %s -p", id)
		output, err := ExecuteCmd(client, cmd)
		if err != nil {
			return state, fmt.Errorf("sacct execution for job %s failed: %v\nOutput: %s", id, err, output)
		}

		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) < 2 {
			return state, fmt.Errorf("not enough lines to parse")
		}
		fields := strings.Split(lines[1], "|") // take the first data line and split by the pipe
		if len(fields) >= 7 {
			state = fields[5]
		} else {
			return state, fmt.Errorf("not enough fields to parse")
		}

		if state == "PENDING" { // when afterok dependency is used (job has to be cancelled)
			reason, err := GetPendingReasonV2(client, id)
			if err != nil {
				return state, err
			}
			fmt.Printf("Job State: PENDING (%s)\n", reason)
			if reason == "DependencyNeverSatisfied" {
				return "PENDING (DependencyNeverSatisfied)", nil
			}
		} else if state != "PENDING" && state != "PREEMPTED" && state != "RUNNING" && state != "SUSPENDED" {
			fmt.Println("Job State:", state)
			return state, nil
		} else {
			fmt.Println("Job State:", state)
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
	return state, nil
}

func ExtractJobDetailsV2(client *ssh.Client, id string) (string, error) {
	jobName := ""
	partition := ""
	state := ""
	exitCode := ""
	cmd := fmt.Sprintf("sacct -j %s -p", id)
	output, err := ExecuteCmd(client, cmd)
	if err != nil {
		return "", fmt.Errorf("sacct execution for job %s failed: %v\nOutput: %s", id, err, output)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("not enough lines to parse")
	}
	fields := strings.Split(lines[1], "|") // take the first data line and split by the pipe
	if len(fields) >= 7 {
		jobName = fields[1]
		partition = fields[2]
		state = fields[5]
		exitCode = fields[6]
	} else {
		return "", fmt.Errorf("not enough fields to parse")
	}
	return fmt.Sprintf("JobID:%s|JobName:%s|Partition:%s|State:%s|ExitCode:%s", id, jobName, partition, state, exitCode), nil
}

func SaveJobStatusesV2(client *ssh.Client, ids []string, file string) error {
	statuses := ""
	for _, id := range ids {
		status, err := ExtractJobDetailsV2(client, id)
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
