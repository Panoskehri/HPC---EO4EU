package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

func GetBatchScripts(schema Schema, submitTemplate string, jobTemplate string) ([]string, error) {
	// Create directory to hold workflow's batch scripts
	err := os.Mkdir(schema.WorkflowID, 0755)
	if err != nil {
		return nil, fmt.Errorf("error creating dir %s: %v", schema.WorkflowID, err)
	}
	batchScripts := make([]string, 0, len(schema.Steps)+1) // this will be returned

	// Creating submission batch script
	tmpl, err := template.ParseFiles(submitTemplate)
	if err != nil {
		return nil, fmt.Errorf("error parsing template for %s: %v", submitTemplate, err)
	}
	batchScripts = append(batchScripts, filepath.Join(schema.WorkflowID, "submit.job"))
	f, err := os.OpenFile(batchScripts[0], os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return nil, fmt.Errorf("error opening file  %s: %v", batchScripts[0], err)
	}
	defer f.Close()
	if err := tmpl.Execute(f, schema); err != nil {
		return nil, fmt.Errorf("error executing template for %s: %v", batchScripts[0], err)
	}

	// Create batch scripts for each step
	steps := schema.Steps
	for i, step := range steps {
		stepSchema := StepBatch{Step: step, WorkflowID: schema.WorkflowID, Account: schema.Account, Workdir: schema.Workdir, Homedir: schema.Homedir}
		batchScripts = append(batchScripts, filepath.Join(schema.WorkflowID, fmt.Sprintf("step_%d.job", i)))
		tmpl, err := template.ParseFiles(jobTemplate)
		if err != nil {
			return nil, fmt.Errorf("error parsing template for %s: %v", jobTemplate, err)
		}
		f, err := os.OpenFile(batchScripts[i+1], os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return nil, fmt.Errorf("error opening file %s: %v", batchScripts[i+1], err)
		}
		defer f.Close()
		if err := tmpl.Execute(f, stepSchema); err != nil {
			return nil, fmt.Errorf("error executing template for %s: %v", batchScripts[i+1], err)
		}
	}

	return batchScripts, nil
}

func GetImages(schema Schema) []string {
	images := make([]string, 0, len(schema.Steps))

	steps := schema.Steps
	for _, step := range steps {
		images = append(images, step.ImageRegistry)
	}

	return images
}

func GetDatalocation(schema Schema) []string {
	return []string{schema.DataLocation}
}

func GetInputs(schema Schema) []string {
	inputs := make([]string, 0, len(schema.Steps))

	steps := schema.Steps
	for _, step := range steps {
		inputs = append(inputs, step.Input)
	}

	return inputs
}

func GetOutputs(schema Schema) []string {
	outputs := make([]string, 0, len(schema.Steps))
	workdir, workflowID := schema.Workdir, schema.WorkflowID

	steps := schema.Steps
	for _, step := range steps {
		outputs = append(outputs, fmt.Sprintf("%s/%s/%s", workdir, workflowID, step.Output))
	}

	return outputs
}
