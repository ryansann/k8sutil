# k8sdump

## Overview

This command can be used to dump Kubernetes resources from a cluster. It reads a config file to determine where your kubeconfig 
file is and what resources you want dumped. I built this for debugging and dumping related k8s resources for analysis.

## Build

Use: `make build`

## Run

If you want to use the default config file location, `./k8sdump.yaml` then you can use: `make run`

Otherwise you can use: 
- `make build`
- `k8sdump --config-file <path>`
