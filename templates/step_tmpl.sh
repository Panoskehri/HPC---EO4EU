#!/bin/bash

#SBATCH --job-name={{.Step.Directives.JobName}}      
#SBATCH --output={{.Workdir}}/{{.WorkflowID}}/result_%j.log      
#SBATCH --error={{.Workdir}}/{{.WorkflowID}}/result_%j.err      
#SBATCH --ntasks={{.Step.Directives.Tasks}}                 
#SBATCH --cpus-per-task={{.Step.Directives.CpusPerTask}}           
#SBATCH --time={{.Step.Directives.Time}}            
#SBATCH --partition={{.Step.Directives.Partition}}
#SBATCH --account={{.Account}}
#SBATCH --mem={{.Step.Directives.Memory}}
#SBATCH --nodes={{.Step.Directives.Nodes}}                 

export APPTAINERENV_DATASET="{{.Workdir}}/{{.WorkflowID}}/{{.Step.Input}}"
export APPTAINERENV_OUTPUT="{{.Workdir}}/{{.WorkflowID}}/{{.Step.Output}}"
{{range $key, $value := .Step.OptionalEnvVars -}}
export APPTAINERENV_{{$key}}="{{$value}}"
{{end}}

srun singularity run -B /users,/work {{.Workdir}}/{{.WorkflowID}}/{{.Step.Image}}
