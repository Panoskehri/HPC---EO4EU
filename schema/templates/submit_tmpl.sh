#!/bin/bash

#SBATCH --output={{.Workdir}}/{{.WorkflowID}}/result_%j.log      
#SBATCH --error={{.Workdir}}/{{.WorkflowID}}/result_%j.err      
#SBATCH --account={{.Account}}
#SBATCH --partition={{.Partition}}
#SBATCH --nodes=1 
#SBATCH --mem={{.Memory}}

{{range $i, $step := .Steps -}}
export STEP{{$i}}="{{$.Workdir}}/{{$.WorkflowID}}/step_{{$i}}.sh"
{{end}}

{{range $i, $step := .Steps -}}
if [[ ! -f "$STEP{{$i}}" ]]; then
    echo "ERROR: $STEP{{$i}} is missing!" >&2
    exit 1
fi
{{end}}

PREV_JOB_ID=""
{{range $i, $step := .Steps}}
if [ -z "$PREV_JOB_ID" ]; then
    SUBMIT_OUTPUT=$(sbatch "$STEP{{$i}}")
else
    SUBMIT_OUTPUT=$(sbatch --dependency=afterok:$PREV_JOB_ID "$STEP{{$i}}")
fi
CURRENT_JOB_ID=$(echo $SUBMIT_OUTPUT | awk '{print $4}')
echo "Step {{$i}} submitted with Job ID: $CURRENT_JOB_ID {{if (gt $i 0)}}(Waiting for $PREV_JOB_ID){{end}}"
PREV_JOB_ID=$CURRENT_JOB_ID
{{end}}