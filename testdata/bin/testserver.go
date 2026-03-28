// testserver is a minimal HTTP/UDP server for integration tests.
// It serves HTTP requests and echoes UDP packets.
//
// Usage:
//
//	testserver [flags]
//
// Flags:
//
//	-http=:80    HTTP listen address (empty to disable)
//	-udp=:5353   UDP listen address (empty to disable)
//
// Build for all platforms:
//
//	make test-server
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

var (
	httpAddr = flag.String("http", ":80", "HTTP listen address (empty to disable)")
	udpAddr  = flag.String("udp", "", "UDP listen address (empty to disable)")
)

func main() {
	flag.Parse()

	// Channel to wait for shutdown
	done := make(chan bool)
	errors := make(chan error, 2)

	// Start HTTP server
	if *httpAddr != "" {
		go func() {
			fmt.Printf("HTTP server listening on %s\n", *httpAddr)
			mux := http.NewServeMux()
			mux.HandleFunc("/", handleHTTP)
			if err := http.ListenAndServe(*httpAddr, mux); err != nil {
				errors <- fmt.Errorf("HTTP server error: %w", err)
			}
		}()
	}

	// Start UDP server
	if *udpAddr != "" {
		go func() {
			fmt.Printf("UDP server listening on %s\n", *udpAddr)
			if err := serveUDP(*udpAddr); err != nil {
				errors <- fmt.Errorf("UDP server error: %w", err)
			}
		}()
	}

	// Handle signals
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sig:
		fmt.Println("Shutting down...")
	case err := <-errors:
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	case <-done:
	}
}

func handleHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, "Method: %s\n", r.Method)
	fmt.Fprintf(w, "URL: %s\n", r.URL.String())
	fmt.Fprintf(w, "Host: %s\n", r.Host)
	fmt.Fprintf(w, "RemoteAddr: %s\n", r.RemoteAddr)

	fmt.Fprintln(w, "\nHeaders:")
	for name, values := range r.Header {
		for _, value := range values {
			fmt.Fprintf(w, "  %s: %s\n", name, value)
		}
	}

	if r.Body != nil && r.ContentLength > 0 {
		fmt.Fprintln(w, "\nBody:")
		io.Copy(w, r.Body)
	}
}

func serveUDP(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("resolve UDP address: %w", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen UDP: %w", err)
	}
	defer conn.Close()

	buf := make([]byte, 65535)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "UDP read error: %v\n", err)
			continue
		}

		// Echo back the received data
		_, err = conn.WriteToUDP(buf[:n], remoteAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "UDP write error: %v\n", err)
		}
	}
}
