package server

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestRunCompletesInflightRequestDuringShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	httpServer := &http.Server{
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			close(requestStarted)
			<-releaseRequest
			writer.WriteHeader(http.StatusNoContent)
		}),
		ReadHeaderTimeout: time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	runResult := make(chan error, 1)
	go func() {
		runResult <- Run(
			ctx,
			httpServer,
			listener,
			time.Second,
			slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)),
		)
	}()

	responseResult := make(chan *http.Response, 1)
	requestError := make(chan error, 1)
	go func() {
		response, requestErr := http.Get("http://" + listener.Addr().String())
		if requestErr != nil {
			requestError <- requestErr
			return
		}
		responseResult <- response
	}()

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	close(releaseRequest)

	select {
	case requestErr := <-requestError:
		t.Fatalf("request error: %v", requestErr)
	case response := <-responseResult:
		defer response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("status = %d", response.StatusCode)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("request did not finish")
	}

	select {
	case runErr := <-runResult:
		if runErr != nil {
			t.Fatalf("Run() error = %v", runErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not finish")
	}
}

func TestNormalizeServeError(t *testing.T) {
	if err := normalizeServeError(http.ErrServerClosed); err != nil {
		t.Fatalf("normalizeServeError(http.ErrServerClosed) = %v", err)
	}
	if err := normalizeServeError(context.Canceled); err == nil {
		t.Fatal("normalizeServeError(context.Canceled) = nil")
	}
}
