package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServiceStatusSlowBodyTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":`))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	s := statusService(server.URL, "")
	defer s.client.CloseIdleConnections()
	s.probeTimeout = 50 * time.Millisecond
	view, err := s.Get(context.Background())
	if err != nil || view.Services[0].Status != "timeout" {
		t.Fatal(view, err)
	}
}

func TestServiceStatusIncompleteBodyUnhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	s := statusService(server.URL, "")
	defer s.client.CloseIdleConnections()
	view, err := s.Get(context.Background())
	if err != nil || view.Services[0].Status != "unhealthy" {
		t.Fatal(view, err)
	}
}

func TestServiceStatusOversizeHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Oversize", strings.Repeat("x", 32*1024))
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	s := statusService(server.URL, "")
	defer s.client.CloseIdleConnections()
	view, err := s.Get(context.Background())
	if err != nil || view.Services[0].Status == "healthy" || strings.Contains(view.Services[0].Message, "X-Oversize") {
		t.Fatal(view, err)
	}
}

func TestServiceStatusQdrantWithoutOptionalKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("api-key") != "" {
			t.Error("unexpected credential")
		}
		if r.URL.Path == "/readyz" {
			_, _ = w.Write([]byte("all shards are ready\n"))
		} else {
			_, _ = w.Write([]byte(`{"status":"ok","result":{"collections":[]}}`))
		}
	}))
	defer server.Close()
	s := statusService("", server.URL)
	defer s.client.CloseIdleConnections()
	s.apiKey = ""
	view, err := s.Get(context.Background())
	if err != nil || view.Services[1].Status != "healthy" {
		t.Fatal(view, err)
	}
}
