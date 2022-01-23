#!/usr/bin/env bash

set -e

go mod tidy
go fmt
go generate
go build
