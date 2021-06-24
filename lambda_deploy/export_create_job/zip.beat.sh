#!/bin/bash
set -eo pipefail

GOOS=linux go build main.go
zip beat.zip main
