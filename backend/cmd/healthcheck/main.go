// Command healthcheck probes the running Subprime server for the container
// HEALTHCHECK instruction (Spec.md §6). The runtime image is minimal, so the
// probe is a small static Go binary instead of a shell/curl invocation.
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// DefaultPort matches the server default in Spec.md §6.
const DefaultPort = "8080"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		os.Exit(1)
	}
}

func run() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = DefaultPort
	}
	client := &http.Client{Timeout: 3 * time.Second}
	return probe(client, healthURL(port))
}

// probe performs the GET and treats any non-200 response as a failure.
func probe(client *http.Client, url string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

// healthURL builds the (unauthenticated) health endpoint URL.
func healthURL(port string) string {
	return fmt.Sprintf("http://localhost:%s/api/health", port)
}
