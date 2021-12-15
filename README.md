<!--
SPDX-FileCopyrightText: 2013 go-bench AUTHORS
SPDX-License-Identifier: CC-BY-4.0
-->
<!-- markdownlint-disable MD041-->
[![Go Report Card](https://goreportcard.com/badge/github.com/EricNeid/go-bench?style=flat-square)](https://goreportcard.com/report/github.com/EricNeid/go-getdockerimage)
![Go](https://github.com/EricNeid/go-bench/workflows/Go/badge.svg)
[![Release](https://img.shields.io/github/release/EricNeid/go-bench.svg?style=flat-square)](https://github.com/EricNeid/go-bench/releases/latest)
[![Gitpod Ready-to-Code](https://img.shields.io/badge/Gitpod-Ready--to--Code-blue?logo=gitpod)](https://gitpod.io/#https://github.com/EricNeid/go-bench)

# About

This tool is a simple benchmark tool to test the performance and throughput of your server.

Forked from <https://github.com/cmpxchg16/gobench>, so be sure to check
their project out as well.

## Quickstart

Checkout the project and run make (given that go build chain is installed.
Hint: You can simply click on the Gitpod link above and compile it online without setting up a tool chain.

```bash
make all
```

## Installation

Simply grep the latest available binary from the release page and put it somewhere in your path.

If go is installed and $GOPATH/bin is in your path, you can download and install the tool directly
by using go install.

Download tool:

```bash
go install github.com/EricNeid/go-bench/cmd/gobench@v0.3.0
```

## Usage

Running HTTP GET:

```bash
gobench -u http://localhost:80 -k=true -c 500 -t 10
```

Running HTTP Post:

```bash
gobench -u http://localhost:80 -k=true -c 500 -t 10 -d ./data.json
gobench -u http://localhost:80 -k=true -c 500 -t 10 -b '{\"name\":\"Timmy\"}'
```

Getting help:

```bash
gobench --help
```

## Question or comments

Please feel free to open a new issue:
<https://github.com/EricNeid/go-bench/issues>
