// SPDX-FileCopyrightText: 2021 Eric Neidhardt
// SPDX-License-Identifier: MIT
package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/EricNeid/go-bench/client"
)

const version = "0.3.0"

var (
	clientCount int = 100

	requestCount        int = -1
	requestsDurationSec int = -1

	url string = ""

	postDataFilePath string = ""
	postBody         string = ""
	contentType      string = ""

	keepAlive bool = false

	clientTimeoutMs int64 = 10 * 1000 // 10 seconds

	authHeader        string = ""
	additionalHeaders string = ""
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Command line options:\n")
		flag.PrintDefaults()
	}
	flag.IntVar(&clientCount, "c", clientCount, "Number of concurrent clients")
	flag.IntVar(&requestCount, "r", requestCount, "Number of requests per client")
	flag.IntVar(&requestsDurationSec, "t", requestsDurationSec, "Duration for performing requests (in seconds)")

	flag.StringVar(&url, "u", url, "URL")

	flag.StringVar(&postDataFilePath, "d", postDataFilePath, "HTTP POST data file path: gobench -u http://localhost -t 10 -d ./data.json")
	flag.StringVar(&postBody, "b", postBody, "HTTP POST body: gobench -u http://localhost -t 10 -b '{\"name\":\"max\"}'")
	flag.StringVar(&contentType, "content-type", contentType, "Content type of post body")

	flag.BoolVar(&keepAlive, "k", keepAlive, "Do HTTP keep-alive ")
	flag.Int64Var(&clientTimeoutMs, "timeout", clientTimeoutMs, "Timeout (in milliseconds)")

	flag.StringVar(&authHeader, "auth", authHeader, "Authorization header: gobench -u http://localhost -t 10 -auth 'Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ=='")
	flag.StringVar(
		&additionalHeaders, "headers", additionalHeaders, "additional header fields: gobench -u http://localhost -t 10 -headers key1=value1,key2=value2",
	)

	flag.Parse()

	if url == "" {
		println("Url is required")
		flag.Usage()
		os.Exit(1)
	}

	if requestCount == -1 && requestsDurationSec == -1 {
		fmt.Println("Request count or request duration must be provided")
		flag.Usage()
		os.Exit(1)
	}

	if requestCount != -1 && requestsDurationSec != -1 {
		fmt.Println("Only one should be provided: [requests|duration]")
		flag.Usage()
		os.Exit(1)
	}

	if clientCount <= 0 {
		fmt.Println("Number of clients must be larger than 0")
		flag.Usage()
		os.Exit(1)
	}
}

func main() {
	request := client.NewRequest(url, postDataFilePath, postBody, contentType, keepAlive, authHeader, additionalHeaders)

	var clients []*client.Client
	for i := 0; i < clientCount; i++ {
		clients = append(clients, client.NewClient(time.Duration(clientTimeoutMs)*time.Millisecond, *request))
	}

	fmt.Printf("Dispatching %d clients\n", len(clients))

	var done sync.WaitGroup
	done.Add(len(clients))
	startTime := time.Now()

	if requestCount != -1 {
		for _, c := range clients {
			go func(c *client.Client) {
				c.RunForAmount(requestCount)
				done.Done()
			}(c)
		}
	} else if requestsDurationSec != -1 {
		for _, c := range clients {
			go func(c *client.Client) {
				c.RunForDuration(time.Duration(requestsDurationSec) * time.Second)
				done.Done()
			}(c)
		}
	}
	fmt.Println("Waiting for results...")
	done.Wait()

	printResults(clients, startTime)
}

func printResults(clients []*client.Client, startTime time.Time) {
	var requests int64
	var success int64
	var failed int64
	var networkFailed int64

	var readThroughput int64
	var writeThroughput int64

	for _, c := range clients {
		requests += int64(c.Statistic.RequestCount)
		success += int64(c.Statistic.SuccessCount)
		networkFailed += int64(c.Statistic.NetworkFailedCount)
		failed += int64(c.Statistic.FailureCount)
		readThroughput += int64(c.Statistic.ReadThroughput)
		writeThroughput += int64(c.Statistic.WriteThroughput)
	}

	elapsed := int64(time.Since(startTime).Seconds())

	if elapsed == 0 {
		elapsed = 1
	}

	fmt.Println()
	fmt.Printf("Requests:                       %10d hits\n", requests)
	fmt.Printf("Successful requests:            %10d hits\n", success)
	fmt.Printf("Network failed:                 %10d hits\n", networkFailed)
	fmt.Printf("Bad requests failed (!2xx):     %10d hits\n", failed)
	fmt.Printf("Successful requests rate:       %10d hits/sec\n", success/elapsed)
	fmt.Printf("Read throughput:                %10d bytes/sec\n", readThroughput/elapsed)
	fmt.Printf("Write throughput:               %10d bytes/sec\n", writeThroughput/elapsed)
	fmt.Printf("Test time:                      %10d sec\n", elapsed)
}
