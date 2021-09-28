package main

import (
	"net"
	"sync"
	"sync/atomic"

	"github.com/valyala/fasthttp"
)

type Result struct {
	requests      int64
	success       int64
	networkFailed int64
	badFailed     int64
}

type MyConn struct {
	net.Conn
}

var readThroughput int64
var writeThroughput int64

// override fasthttp Read function
func (conn *MyConn) Read(b []byte) (n int, err error) {
	len, err := conn.Conn.Read(b)

	if err == nil {
		atomic.AddInt64(&readThroughput, int64(len))
	}

	return len, err
}

// override fasthttp Write function
func (conn *MyConn) Write(b []byte) (n int, err error) {
	len, err := conn.Conn.Write(b)

	if err == nil {
		atomic.AddInt64(&writeThroughput, int64(len))
	}

	return len, err
}

func newMyDialFunction() func(address string) (conn net.Conn, err error) {
	return func(address string) (net.Conn, error) {
		conn, err := net.Dial("tcp", address)
		if err != nil {
			return nil, err
		}

		myConn := &MyConn{Conn: conn}

		return myConn, nil
	}
}

func startClient(configuration runConfiguration, result *Result, done *sync.WaitGroup) {
	// either perform requests until request count is reached or wait for timeout to kick in
	for result.requests < configuration.requestCount || configuration.requestsDuration != -1 {
		for _, tmpUrl := range configuration.urls {

			req := fasthttp.AcquireRequest()

			req.SetRequestURI(tmpUrl)
			req.Header.SetMethodBytes([]byte(configuration.method))

			if configuration.keepAlive {
				req.Header.Set("Connection", "keep-alive")
			} else {
				req.Header.Set("Connection", "close")
			}

			if configuration.contentType != "" {
				req.Header.SetContentType(configuration.contentType)
			}

			for k, v := range configuration.additioanlHeadrs {
				req.Header.Set(k, v)
			}

			req.SetBody(configuration.postData)

			resp := fasthttp.AcquireResponse()
			err := configuration.myClient.Do(req, resp)
			statusCode := resp.StatusCode()
			result.requests++
			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(resp)

			if err != nil {
				result.networkFailed++
				continue
			}

			// check for any success status code
			if statusCode >= 200 && statusCode <= 226 {
				result.success++
			} else {
				result.badFailed++
			}
		}
	}

	done.Done()
}
