#!/usr/bin/env bash

go run ./src/server/cmd/server/main.go &
cd src/web && npm run dev