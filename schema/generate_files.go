package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/template"
)

func GenerateFiles() {
	// Read config file
	file, err := os.ReadFile("config.json")
	if err != nil {
		panic("Error reading config file: " + err.Error())
	}

	// Parse JSON into the Schema struct
	var config Schema
	if err := json.Unmarshal(file, &config); err != nil {
		panic("Error parsing JSON: " + err.Error())
	}

	// Parse and execute the template for the step submission script
	tmpl, err := template.ParseFiles("templates/submit_tmpl.sh")
	if err != nil {
		panic(err)
	}
	f, _ := os.OpenFile("submit.sh", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	defer f.Close()
	if err := tmpl.Execute(f, config); err != nil {
		panic(err)
	}

	// Create batch scripts for each step
	steps := config.Steps
	for i, step := range steps {
		stepConfig := StepBatch{Step: step, WorkflowID: config.WorkflowID, Account: config.Account, Workdir: config.Workdir, Homedir: config.Homedir}
		stepName := fmt.Sprintf("step_%d.sh", i)
		tmpl, err := template.ParseFiles("templates/step_tmpl.sh")
		if err != nil {
			panic(err)
		}
		f, _ := os.OpenFile(stepName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
		defer f.Close()
		if err := tmpl.Execute(f, stepConfig); err != nil {
			panic(err)
		}
	}
}
