package main

import (
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestForwardXTCPRoundTrip(t *testing.T) {
	targetLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer targetLn.Close()
	go func() {
		for {
			conn, err := targetLn.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()

	key := "test-key"
	sec, err := newSecureConn(nil, key)
	if err != nil {
		t.Fatal(err)
	}
	targetPort := targetLn.Addr().(*net.TCPAddr).Port
	exitPort := freeTCPPort(t)
	entryPort := freeTCPPort(t)
	exitDone := make(chan struct{})
	entryDone := make(chan struct{})
	defer close(exitDone)
	defer close(entryDone)

	go func() {
		_ = runExit(exitDone, config{
			Role:       "exit",
			TunnelID:   1,
			ListenPort: exitPort,
			Protocol:   "tcp",
			Key:        key,
		}, sec.aead)
	}()
	waitForTCP(t, exitPort)

	go func() {
		_ = runEntry(entryDone, config{
			Role:       "entry",
			TunnelID:   1,
			RuleID:     2,
			ListenPort: entryPort,
			Protocol:   "tcp",
			ExitHost:   "127.0.0.1",
			ExitPort:   exitPort,
			TargetIP:   "127.0.0.1",
			TargetPort: targetPort,
			Key:        key,
		}, sec.aead)
	}()
	waitForTCP(t, entryPort)

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(entryPort)), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("forwardx")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len("forwardx"))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "forwardx" {
		t.Fatalf("unexpected echo %q", string(buf))
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func waitForTCP(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("port %d did not open", port)
}
