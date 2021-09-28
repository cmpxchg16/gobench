package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

// cli arguments
var (
	requestCount     int64
	requestsDuration int64
	clients          int
	url              string
	urlsFilePath     string
	keepAlive        bool
	postDataFilePath string
	postBody         string
	contentType      string
	writeTimeout     int
	readTimeout      int
	authHeader       string
)

// configuration from cli arguments
type runConfiguration struct {
	urls             []string
	method           string
	postData         []byte
	contentType      string
	requestCount     int64
	requestsDuration int64
	keepAlive        bool
	authHeader       string

	myClient fasthttp.Client
}

func init() {
	flag.Int64Var(&requestCount, "r", -1, "Number of requests per client")
	flag.IntVar(&clients, "c", 100, "Number of concurrent clients")
	flag.StringVar(&url, "u", "", "URL")
	flag.StringVar(&urlsFilePath, "f", "", "URL's file path (line seperated)")
	flag.BoolVar(&keepAlive, "k", true, "Do HTTP keep-alive ")
	flag.StringVar(&postDataFilePath, "d", "", "HTTP POST data file path: gobench -u http://localhost -t 10 -d ./data.json")
	flag.StringVar(&postBody, "b", "", "HTTP POST body: gobench -u http://localhost -t 10 -b '{\"name\":\"max\"}'")
	flag.StringVar(&contentType, "content-type", "", "Content type of post body")
	flag.Int64Var(&requestsDuration, "t", -1, "Period of time (in seconds)")
	flag.IntVar(&writeTimeout, "tw", 5000, "Write timeout (in milliseconds)")
	flag.IntVar(&readTimeout, "tr", 5000, "Read timeout (in milliseconds)")
	flag.StringVar(&authHeader, "auth", "", "Authorization header: gobench -u http://localhost -t 10 -auth 'Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ=='")
	flag.Parse()
}

func main() {
	startTime := time.Now()
	var done sync.WaitGroup
	results := make(map[int]*Result)

	runConfiguration := newRunConfiguration()
	goMaxProcs := os.Getenv("GOMAXPROCS")
	if goMaxProcs == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	// interupt and print results on ctr+c
	Interrupted := make(chan os.Signal, 1)
	signal.Notify(Interrupted, os.Interrupt)

	// register timeout
	timeout := make(chan bool, 1)
	if runConfiguration.requestsDuration != -1 {
		go func() {
			time.Sleep(time.Duration(runConfiguration.requestsDuration) * time.Second)
			timeout <- true
		}()
	}

	go func() {
		select {
		case <-Interrupted:
		case <-timeout:
		}
		printResults(results, startTime)
		os.Exit(0)
	}()

	fmt.Printf("Dispatching %d clients\n", clients)

	done.Add(clients)
	for i := 0; i < clients; i++ {
		result := &Result{}
		results[i] = result
		go startClient(runConfiguration, result, &done)

	}
	fmt.Println("Waiting for results...")
	done.Wait()
	printResults(results, startTime)
}

func newRunConfiguration() runConfiguration {
	if urlsFilePath == "" && url == "" {
		flag.Usage()
		os.Exit(1)
	}

	if requestCount == -1 && requestsDuration == -1 {
		fmt.Println("Requests or period must be provided")
		flag.Usage()
		os.Exit(1)
	}

	if requestCount != -1 && requestsDuration != -1 {
		fmt.Println("Only one should be provided: [requests|period]")
		flag.Usage()
		os.Exit(1)
	}

	configuration := runConfiguration{
		urls:             make([]string, 0),
		method:           "GET",
		postData:         nil,
		keepAlive:        keepAlive,
		authHeader:       authHeader,
		requestCount:     requestCount,
		requestsDuration: requestsDuration,
	}

	if urlsFilePath != "" {
		fileLines, err := readLines(urlsFilePath)

		if err != nil {
			log.Fatalf("Error in ioutil.ReadFile for file: %s Error: %v", urlsFilePath, err)
		}

		configuration.urls = fileLines
	}

	if url != "" {
		configuration.urls = append(configuration.urls, url)
	}

	if postDataFilePath != "" {
		configuration.method = "POST"
		data, err := os.ReadFile(postDataFilePath)
		if err != nil {
			log.Fatalf("Error in ioutil.ReadFile for file path: %s Error: %v", postDataFilePath, err)
		}
		configuration.postData = data
	}

	if postBody != "" {
		configuration.method = "POST"
		configuration.postData = []byte(postBody)
	}

	if contentType != "" {
		configuration.contentType = contentType
	}

	configuration.myClient.ReadTimeout = time.Duration(readTimeout) * time.Millisecond
	configuration.myClient.WriteTimeout = time.Duration(writeTimeout) * time.Millisecond
	configuration.myClient.MaxConnsPerHost = clients

	configuration.myClient.Dial = newMyDialFunction()

	return configuration
}

func readLines(path string) (lines []string, err error) {
	var file *os.File
	var part []byte
	var prefix bool

	if file, err = os.Open(path); err != nil {
		return
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	buffer := bytes.NewBuffer(make([]byte, 0))
	for {
		if part, prefix, err = reader.ReadLine(); err != nil {
			break
		}
		buffer.Write(part)
		if !prefix {
			lines = append(lines, buffer.String())
			buffer.Reset()
		}
	}
	if err == io.EOF {
		err = nil
	}
	return
}

func printResults(results map[int]*Result, startTime time.Time) {
	var requests int64
	var success int64
	var networkFailed int64
	var badFailed int64

	for _, result := range results {
		requests += result.requests
		success += result.success
		networkFailed += result.networkFailed
		badFailed += result.badFailed
	}

	elapsed := int64(time.Since(startTime).Seconds())

	if elapsed == 0 {
		elapsed = 1
	}

	fmt.Println()
	fmt.Printf("Requests:                       %10d hits\n", requests)
	fmt.Printf("Successful requests:            %10d hits\n", success)
	fmt.Printf("Network failed:                 %10d hits\n", networkFailed)
	fmt.Printf("Bad requests failed (!2xx):     %10d hits\n", badFailed)
	fmt.Printf("Successful requests rate:       %10d hits/sec\n", success/elapsed)
	fmt.Printf("Read throughput:                %10d bytes/sec\n", readThroughput/elapsed)
	fmt.Printf("Write throughput:               %10d bytes/sec\n", writeThroughput/elapsed)
	fmt.Printf("Test time:                      %10d sec\n", elapsed)
}
