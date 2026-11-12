package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthURL(t *testing.T) {
	tests := []struct {
		name string
		port string
		want string
	}{
		{name: "default port", port: "8080", want: "http://localhost:8080/api/health"},
		{name: "custom port", port: "9000", want: "http://localhost:9000/api/health"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := healthURL(tc.port); got != tc.want {
				t.Errorf("healthURL(%q) = %q, want %q", tc.port, got, tc.want)
			}
		})
	}
}

func TestProbe(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "ok", status: http.StatusOK, wantErr: false},
		{name: "server error", status: http.StatusInternalServerError, wantErr: true},
		{name: "not found", status: http.StatusNotFound, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer server.Close()

			err := probe(server.Client(), server.URL)
			if tc.wantErr && err == nil {
				t.Error("probe() = nil, want error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("probe() = %v, want nil", err)
			}
		})
	}
}

func TestProbeUnreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	if err := probe(server.Client(), url); err == nil {
		t.Error("probe() on closed server = nil, want error")
	}
}
