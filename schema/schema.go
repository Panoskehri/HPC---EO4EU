package main

type Schema struct {
	WorkflowID   string
	Account      string // accounting project assigned by HPC admins
	Workdir      string // working directory assigned by HPC admins
	Homedir      string // home directory assigned by HPC admins
	DataLocation string // location of input data for first step (transfer to HPC inside the working directory)
	Memory       string // directive required by step submission script
	Partition    string // directive required by step submission script
	Steps        []Step
}

type Step struct {
	Input           string          // input dir
	Output          string          // output dir
	ImageRegistry   string          // location of singularity image (i.e. registry)
	Image           string          // name of the singularity image
	Directives      BatchDirectives // batch file directives required by Slurm
	OptionalEnvVars map[string]string
}

type BatchDirectives struct {
	JobName     string
	Partition   string
	Memory      string
	Nodes       string
	Tasks       string
	CpusPerTask string
	Time        string
}

type StepBatch struct {
	Step       Step
	WorkflowID string
	Account    string
	Workdir    string
	Homedir    string
}
