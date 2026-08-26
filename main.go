// Command mdserver serves the markdown files in a directory as a website.
//
// Run it with no arguments in any folder full of markdown:
//
//	mdserver
//
// The folder is the router and the filename is the page: `docs/setup.md`
// becomes `/docs/setup`. Files are read and converted on every request, so
// editing a file and reloading is the whole workflow.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bizshuk/mdserver/svc/server"
	"github.com/bizshuk/mdserver/svc/site"
)

const (
	// APP_NAME is the directory mdserver owns under ~/.config, following the
	// workspace convention that puts an issued certificate pair at
	// ~/.config/<app>/certs/<app>.{crt,key}.
	APP_NAME = "mdserver"

	// DEFAULT_PORT is used when none is given. If it is taken, mdserver walks
	// forward to keep the zero-argument case working.
	DEFAULT_PORT = 8080
	// PORT_SCAN_LIMIT bounds that walk.
	PORT_SCAN_LIMIT = 20

	READ_HEADER_TIMEOUT = 10 * time.Second
	SHUTDOWN_TIMEOUT    = 5 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mdserver:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		addrFlag  = flag.String("addr", "", "listen address, e.g. :8080 or 127.0.0.1:3000")
		portFlag  = flag.Int("port", 0, "listen port (overrides -addr; default: first free port from 8080)")
		rootFlag  = flag.String("root", ".", "directory to serve")
		titleFlag = flag.String("title", "", "site title (defaults to the directory name)")
		quietFlag = flag.Bool("quiet", false, "only log warnings and errors")
		tlsFlag   = flag.Bool("tls", false, "serve HTTPS with ~/.config/mdserver/certs/mdserver.{crt,key}")
		certFlag  = flag.String("cert", "", "TLS certificate file (implies -tls)")
		keyFlag   = flag.String("key", "", "TLS private key file (implies -tls)")
	)
	flag.Usage = usage
	flag.Parse()

	// A bare positional argument is the directory, so `mdserver ./docs` works.
	root := *rootFlag
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}

	level := slog.LevelInfo
	if *quietFlag {
		level = slog.LevelWarn
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	// Resolved before anything binds, so a missing or unreadable certificate
	// fails while the port is still free.
	certificate, scheme, err := loadCertificate(*tlsFlag, *certFlag, *keyFlag)
	if err != nil {
		return err
	}

	contentSite, err := site.New(root)
	if err != nil {
		return err
	}
	if *titleFlag != "" {
		contentSite.Title = *titleFlag
	}

	srv, err := server.New(contentSite, logger)
	if err != nil {
		return err
	}

	listenAddr := *addrFlag
	if *portFlag != 0 {
		listenAddr = ":" + strconv.Itoa(*portFlag)
	}
	listener, err := listen(listenAddr)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: READ_HEADER_TIMEOUT,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
	if certificate != nil {
		listener = tls.NewListener(listener, &tls.Config{
			Certificates: []tls.Certificate{*certificate},
			MinVersion:   tls.VersionTLS12,
		})
	}

	fmt.Printf("mdserver  serving %s\n          on %s://%s\n", contentSite.Root, scheme, displayAddr(listener.Addr()))

	return serve(httpServer, listener, logger)
}

// loadCertificate resolves the pair mdserver should present, returning a nil
// certificate when it stays on plain HTTP.
//
// With no explicit paths, -tls means the pair the local CA issues by
// convention: `issue.sh mdserver` in ~/projects/platform/inf/pkg/tls writes
// ~/.config/mdserver/certs/mdserver.{crt,key}, so the flag needs no argument.
// Passing -cert/-key points at any other pair and implies -tls.
func loadCertificate(enabled bool, certFile, keyFile string) (*tls.Certificate, string, error) {
	switch {
	case certFile == "" && keyFile == "":
		if !enabled {
			return nil, "http", nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, "", fmt.Errorf("locate home directory: %w", err)
		}
		dir := filepath.Join(home, ".config", APP_NAME, "certs")
		certFile = filepath.Join(dir, APP_NAME+".crt")
		keyFile = filepath.Join(dir, APP_NAME+".key")
	case certFile == "" || keyFile == "":
		return nil, "", errors.New("-cert and -key must be given together")
	}

	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, "", fmt.Errorf("load certificate %s: %w", certFile, err)
	}
	return &certificate, "https", nil
}

// serve runs the server until an interrupt arrives, then drains in-flight
// requests before returning.
func serve(httpServer *http.Server, listener net.Listener, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(listener)
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), SHUTDOWN_TIMEOUT)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}

// listen binds addr, or scans forward from DEFAULT_PORT when addr is empty so
// that running mdserver twice does not fail on a busy port.
func listen(addr string) (net.Listener, error) {
	if addr != "" {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("listen on %s: %w", addr, err)
		}
		return listener, nil
	}

	var lastErr error
	for port := DEFAULT_PORT; port < DEFAULT_PORT+PORT_SCAN_LIMIT; port++ {
		listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err == nil {
			return listener, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("no free port in %d-%d: %w", DEFAULT_PORT, DEFAULT_PORT+PORT_SCAN_LIMIT-1, lastErr)
}

// displayAddr turns a wildcard bind address into something clickable.
func displayAddr(addr net.Addr) string {
	text := addr.String()
	if strings.HasPrefix(text, "[::]:") {
		return "localhost:" + strings.TrimPrefix(text, "[::]:")
	}
	if strings.HasPrefix(text, "0.0.0.0:") {
		return "localhost:" + strings.TrimPrefix(text, "0.0.0.0:")
	}
	return text
}

func usage() {
	fmt.Fprint(flag.CommandLine.Output(), `mdserver renders the markdown in a directory as a website.

Usage:
  mdserver [flags] [directory]

The folder is the router and the filename is the page:
  ./README.md          ->  /
  ./docs/setup.md      ->  /docs/setup
  ./docs/              ->  /docs/   (listing, with docs/README.md on top)

HTTPS:
  mdserver -tls        uses ~/.config/mdserver/certs/mdserver.{crt,key},
                       the path the local CA issues to; reach it at
                       https://localhost:<port> or https://mdserver.local:<port>

Flags:
`)
	flag.PrintDefaults()
}
