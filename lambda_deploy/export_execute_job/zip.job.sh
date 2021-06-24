#!/bin/bash
set -eo pipefail

GOOS=linux go build main.go
zip job.zip main
