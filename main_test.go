package main

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCommandPortFlag(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}

	binary := filepath.Join(t.TempDir(), "mdserver")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build mdserver: %v\n%s", err, output)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "-port", strconv.Itoa(port), t.TempDir())
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
		}
	})

	wantURL := "http://localhost:" + strconv.Itoa(port)
	foundURL := false
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), wantURL) {
			foundURL = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !foundURL {
		err := cmd.Wait()
		t.Fatalf("mdserver did not listen on requested port: %v\n%s", err, stderr.String())
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second)
	if err != nil {
		t.Fatalf("connect to requested port: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("stop mdserver: %v\n%s", err, stderr.String())
	}
}
