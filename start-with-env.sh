#!/bin/bash

export CONTINUUM_GRPC_URL="100.24.216.168:9090"
export CONTINUUM_REST_URL="http://100.24.216.168:8080/api/v1"

exec go run ./cmd/gateway/main.go
