#!/bin/bash
#SBATCH -N 1
#SBATCH -n 1
#SBATCH -t 00:01:00
#SBATCH -o job-%j.out
#SBATCH --job-name=testing_scp

hostname
echo "Hello from Slurm"
python3 /home/panos/job_script.py