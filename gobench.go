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
	"sync"
	"time"
        "strings"
        "math/rand"
        "strconv"
        "errors"
)

var (
	requests         int64
	period           int64
	clients          int
	url              string
	urlsFilePath     string
	keepAlive        bool
	postDataFilePath string
	connectTimeout   int
	writeTimeout     int
	readTimeout      int
)
const (
        maxExpandNum = 1000
)
type Configuration struct {
	urls      []string
	method    string
	postData  []byte
	requests  int64
	period    int64
	keepAlive bool
}

type Result struct {
	requests        int64
	success         int64
	networkFailed   int64
	badFailed       int64
	readThroughput  int64
	writeThroughput int64
}

type MyConn struct {
	net.Conn
	readTimeout  time.Duration
	writeTimeout time.Duration
	result       *Result
}

func (this *MyConn) Read(b []byte) (n int, err error) {
	len, err := this.Conn.Read(b)

	if err == nil {
		this.result.readThroughput += int64(len)
		this.Conn.SetReadDeadline(time.Now().Add(this.readTimeout))
	}

	return len, err
}

func (this *MyConn) Write(b []byte) (n int, err error) {
	len, err := this.Conn.Write(b)

	if err == nil {
		this.result.writeThroughput += int64(len)
		this.Conn.SetWriteDeadline(time.Now().Add(this.writeTimeout))
	}

	return len, err
}

func init() {
	flag.Int64Var(&requests, "r", -1, "Number of requests per client")
	flag.IntVar(&clients, "c", 100, "Number of concurrent clients")
        flag.StringVar(&url, "u", "", fmt.Sprintf(`URL. Support expression such as {S10,1-100}  {R14,2-9}.
            S or s means sequence number between [1,100] , the amount is 10, as specified in the expression.
            R or r means random number between [2,9] , the amount is 14. 
            We will generate at most %d urls one time, for the security reason.
            For example : http://www.qq.com/?{s3,1-10} will produce 3 lines as follow
            http://www.qq.com?1
            http://www.qq.com?2
            http://www.qq.com?3 `, maxExpandNum))
	flag.StringVar(&urlsFilePath, "f", "", "URL's file path (line seperated)")
	flag.BoolVar(&keepAlive, "k", true, "Do HTTP keep-alive")
	flag.StringVar(&postDataFilePath, "d", "", "HTTP POST data file path")
	flag.Int64Var(&period, "t", -1, "Period of time (in seconds)")
	flag.IntVar(&connectTimeout, "tc", 5000, "Connect timeout (in milliseconds)")
	flag.IntVar(&writeTimeout, "tw", 5000, "Write timeout (in milliseconds)")
	flag.IntVar(&readTimeout, "tr", 5000, "Read timeout (in milliseconds)")
}

func printResults(results map[int]*Result, startTime time.Time) {
	var requests int64
	var success int64
	var networkFailed int64
	var badFailed int64
	var readThroughput int64
	var writeThroughput int64

	for _, result := range results {
		requests += result.requests
		success += result.success
		networkFailed += result.networkFailed
		badFailed += result.badFailed
		readThroughput += result.readThroughput
		writeThroughput += result.writeThroughput
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
        fmt.Printf("Read transferred:               %10d bytes\n", readThroughput)  // add to contrast it with ab
        fmt.Printf("Write transferred:              %10d bytes\n", writeThroughput)
	fmt.Printf("Read Speed:                     %10d bytes/sec\n", readThroughput/elapsed)
	fmt.Printf("Write Speed:                    %10d bytes/sec\n", writeThroughput/elapsed)
	fmt.Printf("Test time:                      %10d sec\n", elapsed)
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

	configuration := &Configuration{
		urls:      make([]string, 0),
		method:    "GET",
		postData:  nil,
		keepAlive: keepAlive,
		requests:  int64((1 << 63) - 1)}

	if period != -1 {
		configuration.period = period

		timeout := make(chan bool, 1)
		go func() {
			<-time.After(time.Duration(period) * time.Second)
			timeout <- true
		}()

		go func() {
			<-timeout
			pid := os.Getpid()
			proc, _ := os.FindProcess(pid)
			err := proc.Signal(os.Interrupt)
			if err != nil {
				log.Println(err)
				return
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
		//configuration.urls = append(configuration.urls, url)
		configuration.urls = append(configuration.urls, expandUrl(url)...)
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

func TimeoutDialer(result *Result, connectTimeout, readTimeout, writeTimeout time.Duration) func(net, address string) (conn net.Conn, err error) {
	return func(mynet, address string) (net.Conn, error) {
		conn, err := net.DialTimeout(mynet, address, connectTimeout)
		if err != nil {
			return nil, err
		}

		conn.SetReadDeadline(time.Now().Add(readTimeout))
		conn.SetWriteDeadline(time.Now().Add(writeTimeout))

		myConn := &MyConn{Conn: conn, readTimeout: readTimeout, writeTimeout: writeTimeout, result: result}

		return myConn, nil
	}
}

func MyClient(result *Result, connectTimeout, readTimeout, writeTimeout time.Duration) *http.Client {

	return &http.Client{
		Transport: &http.Transport{
			Dial:              TimeoutDialer(result, connectTimeout, readTimeout, writeTimeout),
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: !keepAlive,
			Proxy:             http.ProxyFromEnvironment,
		},
	}
}


// 模板1，开如 S123,1-1000
// 即，以字母开头，紧跟一个数字表示生成个数，逗号，数据范围 from-to
func expression_template_1(e []byte) (n,from,to int, err error) {
    n , from , to ,err = 0,0,0,nil
    if e==nil || len(e)<6 {
        err = errors.New("invalid url expression: length not enough")
        return
    }
    if bytes.IndexAny(e[0:1],"SsRr")==0 {
        comma_cutted := bytes.SplitN(e[1:], []byte{','},2)
        if comma_cutted == nil || len(comma_cutted)<2{
            err = errors.New("invalid url expression: no enough comma seperated fields")
            return
        }
        if n,err = strconv.Atoi( string(comma_cutted[0]) ); err != nil {
            return
        }
        dash_cutted := bytes.SplitN(comma_cutted[1], []byte{'-'}, 2);
        if dash_cutted == nil || len(dash_cutted)<2 {
            err = errors.New("invalid url expression: range invalid");
            return
        }
        if from,err = strconv.Atoi(string(dash_cutted[0])); err != nil {
            return
        }
        if to,err = strconv.Atoi(string(dash_cutted[1])); err != nil {
            return
        }
        return
    }else {
        err = errors.New("invalid url expression: expression not suit for the template1");
        return
    }
}



// 按表达式生成具体的数值
// e.g. : e="R3,0-9" 将会生成3个随机数，范围在[0,9]
// e="S3,0-9" 将会生成 0,1,2 三个顺序数
// 最多生成1000个
func genValue(e []byte) (values []string) {
    values = nil
    if len(e)==0 {
        return
    }
    switch e[0] {
        case 'S' , 's':
            // sequence
            n,from,to,err := expression_template_1(e)
            if err!=nil {
                log.Fatal(err)
            }
            for i:=from;i<=to && i-from<n && i-from<maxExpandNum ;i++ {
                values = append(values, fmt.Sprintf("%d",i))
            }
        case 'R' , 'r':
            // random
            n,from,to,err := expression_template_1(e)
            if err!=nil {
                log.Fatal(err)
            }
            for i:=0;i<n && i<maxExpandNum;i++ {
                values = append(values, fmt.Sprintf("%d",rand.Intn(to-from+1)+from))
            }
        default :
            return
        }
    return
}
// expand the {....} symbol to a value list
// For now, only support sequence expand e.g. {S1,99} or {1,99} ;  rand expand {R1,99} ;
func expandUrl(u string) (ret_urls []string ){
        ret_urls = nil
        url := []byte(u)
        leftBracket,rightBracket := bytes.IndexByte(url, '{'), bytes.IndexByte(url,'}')
        if leftBracket>4 && rightBracket>4 && leftBracket<rightBracket{
            expanded_value := genValue(url[leftBracket+1:rightBracket])
            for i,gen := range expanded_value {
                ret_urls = append(ret_urls,
                    strings.Replace(u,u[leftBracket:rightBracket+1],gen,1) )
                    fmt.Printf( "debug: gen url [%d]:%s\n",i, ret_urls[len(ret_urls)-1])
            }
        } else {
            ret_urls = append(ret_urls,u)
        }
        return
}

func client(configuration *Configuration, result *Result, done *sync.WaitGroup) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("caught recover: ", r)
			os.Exit(1)
		}
	}()

	myclient := MyClient(result, time.Duration(connectTimeout)*time.Millisecond,
		time.Duration(readTimeout)*time.Millisecond,
		time.Duration(writeTimeout)*time.Millisecond)

	for result.requests < configuration.requests {
		for _, tmpUrl := range configuration.urls {
			req, _ := http.NewRequest(configuration.method, tmpUrl, bytes.NewReader(configuration.postData))

			if configuration.keepAlive == true {
				req.Header.Add("Connection", "keep-alive")
			} else {
				req.Header.Add("Connection", "close")
			}

			resp, err := myclient.Do(req)
			result.requests++

			if err != nil {
				result.networkFailed++
				continue
			}

			//_, errRead := ioutil.ReadAll(resp.Body)
                        // use a memory efficient way
                        _m := make([]byte,1024)
                        var errRead error = nil
                        for errRead==nil {
                            _,errRead = resp.Body.Read(_m)
                        }
                        if errRead == io.EOF {
                            errRead = nil
                        }

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

	done.Done()
}

func main() {

	startTime := time.Now()
	var done sync.WaitGroup
	results := make(map[int]*Result)

	signalChannel := make(chan os.Signal, 2)
	signal.Notify(signalChannel, os.Interrupt)
	go func() {
		_ = <-signalChannel
		printResults(results, startTime)
		os.Exit(0)
	}()

	flag.Parse()

	configuration := NewConfiguration()

	goMaxProcs := os.Getenv("GOMAXPROCS")

	if goMaxProcs == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	fmt.Printf("Dispatching %d clients\n", clients)

	done.Add(clients)
	for i := 0; i < clients; i++ {
		result := &Result{}
		results[i] = result
		go client(configuration, result, &done)

	}
	fmt.Println("Waiting for results...")
	done.Wait()
	printResults(results, startTime)
}
