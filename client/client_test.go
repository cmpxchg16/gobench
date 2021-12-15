// SPDX-FileCopyrightText: 2021 Eric Neidhardt
// SPDX-License-Identifier: MIT
package client

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/EricNeid/go-bench/internal/verify"
)

func TestNewRequest(t *testing.T) {
	// action
	result := NewRequest("http://localhost", "", "{\"test\":\"value\"}", "application/json", true, "authorizationHeader", "key1=value1,key2=value2, key3=value3")
	// verify
	verify.NotNil(t, result, "Request is nil")
	verify.Equals(t, "http://localhost", result.Url)
	verify.Equals(t, []byte("{\"test\":\"value\"}"), result.PostBody)
	verify.Equals(t, "application/json", result.ContentType)
	verify.Equals(t, true, result.KeepAlive)
	verify.Equals(t, 4, len(result.AdditionalHeaders))
	verify.Equals(t, "authorizationHeader", result.AdditionalHeaders["Authorization"])
	verify.Equals(t, "value1", result.AdditionalHeaders["key1"])
	verify.Equals(t, "value2", result.AdditionalHeaders["key2"])
	verify.Equals(t, "value3", result.AdditionalHeaders["key3"])
}

func TestPerformRequest_get(t *testing.T) {
	// arrange
	requestReceived := false
	// create test server
	handler := http.NewServeMux()
	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})
	srv := http.Server{Addr: "localhost:8080", Handler: handler}
	go srv.ListenAndServe()
	defer srv.Close()
	// create unit
	unit := Client{
		Request: Request{
			Url: "http://localhost:8080/",
		},
	}
	// action
	unit.PerformRequest()
	// verify
	verify.Assert(t, requestReceived, "Request was not received")
	verify.Equals(t, 1, unit.Statistic.RequestCount)
	verify.Equals(t, 1, unit.Statistic.SuccessCount)
	verify.Equals(t, 0, unit.Statistic.FailureCount)
	verify.Equals(t, 0, unit.Statistic.NetworkFailedCount)
	verify.Equals(t, 0, unit.Statistic.IOFailedCount)
	verify.Equals(t, int64(len([]byte("test response"))), unit.Statistic.ReadThroughput)
	verify.Equals(t, int64(0), unit.Statistic.WriteThroughput)
}

func TestPerformRequestWithContext_shouldCancelRequestAfterDeadline(t *testing.T) {
	// arrange
	// create test server
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(1)*time.Second)
	defer cancel()
	handler := http.NewServeMux()
	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// let client hang until timeout
		<-ctx.Done()
	})
	srv := http.Server{Addr: "localhost:8080", Handler: handler}
	go srv.ListenAndServe()
	defer srv.Close()
	// create unit
	unit := Client{
		Request: Request{
			Url: "http://localhost:8080/",
		},
	}
	// action
	unit.PerformRequestWithContent(ctx)
	// verify
	verify.Equals(t, 1, unit.Statistic.RequestCount)
	verify.Equals(t, 0, unit.Statistic.FailureCount)
	verify.Equals(t, 1, unit.Statistic.NetworkFailedCount)
	verify.Equals(t, 0, unit.Statistic.IOFailedCount)
}

func TestPerformRequest_withCustomHeader(t *testing.T) {
	// arrange
	requestReceived := false
	// create test server
	handler := http.NewServeMux()
	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("custom-header-field") == "test header value" {
			requestReceived = true
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := http.Server{Addr: "localhost:8080", Handler: handler}
	go srv.ListenAndServe()
	defer srv.Close()
	// create unit
	unit := Client{
		Request: Request{
			Url: "http://localhost:8080/",
			AdditionalHeaders: map[string]string{
				"custom-header-field": "test header value",
			},
		},
	}
	// action
	unit.PerformRequest()
	// verify
	verify.Assert(t, requestReceived, "Request was not received")
}

func TestPerformRequest_post(t *testing.T) {
	// arrange
	requestReceived := false
	// create test server
	handler := http.NewServeMux()
	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if body, err := io.ReadAll(r.Body); err == nil && string(body) == "test body" {
			requestReceived = true
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})
	srv := http.Server{Addr: "localhost:8080", Handler: handler}
	go srv.ListenAndServe()
	defer srv.Close()
	// create unit
	unit := Client{
		Request: Request{
			Url:         "http://localhost:8080/",
			PostBody:    []byte("test body"),
			ContentType: "text/string",
		},
	}
	// action
	unit.PerformRequest()
	// verify
	verify.Assert(t, requestReceived, "Request was not received")
	verify.Equals(t, 1, unit.Statistic.RequestCount)
	verify.Equals(t, 1, unit.Statistic.SuccessCount)
	verify.Equals(t, 0, unit.Statistic.FailureCount)
	verify.Equals(t, 0, unit.Statistic.NetworkFailedCount)
	verify.Equals(t, 0, unit.Statistic.IOFailedCount)
	verify.Equals(t, int64(len([]byte("test response"))), unit.Statistic.ReadThroughput)
	verify.Equals(t, int64(len("test body")), unit.Statistic.WriteThroughput)
}

func TestRunForAmount(t *testing.T) {
	// arrange
	receivedCount := 0
	// create test server
	handler := http.NewServeMux()
	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		receivedCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})
	srv := http.Server{Addr: "localhost:8080", Handler: handler}
	go srv.ListenAndServe()
	defer srv.Close()
	// create unit
	unit := Client{
		Request: Request{
			Url: "http://localhost:8080/",
		},
	}
	// action
	unit.RunForAmount(10)
	// verify
	verify.Equals(t, 10, receivedCount)
	verify.Equals(t, 10, unit.Statistic.RequestCount)
	verify.Equals(t, 10, unit.Statistic.SuccessCount)
	verify.Equals(t, 0, unit.Statistic.FailureCount)
	verify.Equals(t, 0, unit.Statistic.NetworkFailedCount)
	verify.Equals(t, 0, unit.Statistic.IOFailedCount)
	verify.Equals(t, 10*int64(len([]byte("test response"))), unit.Statistic.ReadThroughput)
}

func TestRunForDuration(t *testing.T) {
	// arrange
	receivedCount := 0
	// create test server
	handler := http.NewServeMux()
	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
		receivedCount++
	})
	srv := http.Server{Addr: "localhost:8080", Handler: handler}
	go srv.ListenAndServe()
	defer srv.Close()
	// create unit
	unit := Client{
		Request: Request{
			Url: "http://localhost:8080/",
		},
	}
	// action
	unit.RunForDuration(1 * time.Second)
	// verify
	verify.Assert(t, receivedCount > 0, "No request received")
	verify.Assert(t, unit.Statistic.RequestCount > 0, "No request received")
	verify.Equals(t, unit.Statistic.RequestCount, unit.Statistic.SuccessCount)
	verify.Equals(t, int64(unit.Statistic.SuccessCount*len([]byte("test response"))), unit.Statistic.ReadThroughput)
}
