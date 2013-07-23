package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
)

var intrrupted int32 = 0

var (
	requests         int64
	period           int64
	clients          int
	url              string
	urlsFilePath     string
	keepAlive        bool
	postDataFilePath string
	connectTimeout   int
	readWriteTimeout int
)

type Configuration struct {
	client    *http.Client
	urls      []string
	method    string
	postData  []byte
	requests  int64
	period    int64
	keepAlive bool
}

type Result struct {
	requests      int64
	success       int64
	networkFailed int64
	badFailed     int64
}

func init() {
	flag.Int64Var(&requests, "r", -1, "Number of requests per client")
	flag.IntVar(&clients, "c", 100, "Number of concurrent clients")
	flag.StringVar(&url, "u", "", "URL")
	flag.StringVar(&urlsFilePath, "f", "", "URL's file path (line seperated)")
	flag.BoolVar(&keepAlive, "k", true, "Do HTTP keep-alive")
	flag.StringVar(&postDataFilePath, "d", "", "HTTP POST data file path")
	flag.Int64Var(&period, "t", -1, "Period of time (in seconds)")
	flag.IntVar(&connectTimeout, "tc", 5000, "Connect timeout (in milliseconds)")
	flag.IntVar(&readWriteTimeout, "trw", 5000, "Read/Write timeout (in milliseconds)")
}

func printResults(c chan *Result, startTime time.Time) {
	var requests int64
	var success int64
	var networkFailed int64
	var badFailed int64

	for i := 0; i < clients; i++ {
		result := <-c
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
	fmt.Printf("Requests:            %10d hits\n", requests)
	fmt.Printf("Successful requests: %10d hits\n", success)
	fmt.Printf("Network failed:      %10d hits\n", networkFailed)
	fmt.Printf("Bad failed:          %10d hits\n", badFailed)
	fmt.Printf("Requests rate:       %10d hits/sec\n", success / elapsed)
	fmt.Printf("Test time:           %10d sec\n", elapsed)
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

func interrupted() bool {
	return atomic.LoadInt32(&intrrupted) != 0
}

func interrupt() {
	atomic.StoreInt32(&intrrupted, 1)
}

func NewConfiguration() *Configuration {

	if urlsFilePath == "" && url == "" {
		flag.Usage()
		os.Exit(1)
	}

	if requests == -1 && period == -1 {
		fmt.Println("Requests or period must be provided")
		flag.Usage()
		os.Exit(1)
	}

	if requests != -1 && period != -1 {
		fmt.Println("Only one should be provided: [requests|period]")
		flag.Usage()
		os.Exit(1)
	}

	configuration := &Configuration{client: MyClient(time.Duration(connectTimeout)*time.Millisecond, time.Duration(readWriteTimeout)*time.Millisecond),
		urls:      make([]string, 0),
		method:    "GET",
		postData:  nil,
		keepAlive: keepAlive,
		requests:  int64((1 << 63) - 1)}

	if period != -1 {
		configuration.period = period

		timeout := make(chan bool, 1)
		go func() {
			time.Sleep(time.Duration(period) * time.Second)
			timeout <- true
		}()

		go func() {
			select {
			case <-timeout:
				interrupt()
			}
		}()
	}

	if requests != -1 {
		configuration.requests = requests
	}

	if urlsFilePath != "" {
		fileLines, err := readLines(urlsFilePath)

		if err != nil {
			log.Fatalf("Error in ioutil.ReadFile for file: %s Error: ", urlsFilePath, err)
		}

		configuration.urls = fileLines
	}

	if url != "" {
		configuration.urls = append(configuration.urls, url)
	}

	if postDataFilePath != "" {
		configuration.method = "POST"

		data, err := ioutil.ReadFile(postDataFilePath)

		if err != nil {
			log.Fatalf("Error in ioutil.ReadFile for file path: %s Error: ", postDataFilePath, err)
		}

		configuration.postData = data
	}

	return configuration
}

func TimeoutDialer(connectTimeout, readWriteTimeout time.Duration) func(net, address string) (conn net.Conn, err error) {
	return func(mynet, address string) (net.Conn, error) {
		conn, err := net.DialTimeout(mynet, address, connectTimeout)
		if err != nil {
			return nil, err
		}

		conn.SetDeadline(time.Now().Add(readWriteTimeout))
		return conn, nil
	}
}

func MyClient(connectTimeout, readWriteTimeout time.Duration) *http.Client {

	return &http.Client{
		Transport: &http.Transport{
			Dial:            TimeoutDialer(connectTimeout, readWriteTimeout),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

func client(configuration *Configuration, c chan *Result) {

	result := &Result{requests: 0, success: 0, networkFailed: 0, badFailed: 0}

	for result.requests < configuration.requests && !interrupted() {
		for _, tmpUrl := range configuration.urls {
			req, _ := http.NewRequest(configuration.method, tmpUrl, bytes.NewReader(configuration.postData))

			if configuration.keepAlive == true {
				req.Header.Add("Connection", "keep-alive")
			} else {
				req.Header.Add("Connection", "close")
			}

			resp, err := configuration.client.Do(req)
			result.requests++

			if err != nil {
				result.networkFailed++
				continue
			}

			_, errRead := ioutil.ReadAll(resp.Body)

			if errRead != nil {
				result.networkFailed++
				continue
			}

			if resp.StatusCode == http.StatusOK {
				result.success++
			} else {
				result.badFailed++
			}

			resp.Body.Close()
		}
	}	

	c <- result
}

func main() {

	signalChannel := make(chan os.Signal, 2)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		_ = <-signalChannel
		interrupt()
	}()

	flag.Parse()

	configuration := NewConfiguration()

	runtime.GOMAXPROCS(runtime.NumCPU())

	resultChannel := make(chan *Result)

	fmt.Printf("Dispatching %d clients\n", clients)
	
	startTime := time.Now()
	for i := 0; i < clients; i++ {

		go client(configuration, resultChannel)

	}
	fmt.Println("Waiting for results...")
	printResults(resultChannel, startTime)
}
