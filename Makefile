# SPDX-FileCopyrightText: 2021 Eric Neidhardt
# SPDX-License-Identifier: CC0-1.0

all: test build

test:
	go test ./...

build:
	go build ./cmd/gobench
