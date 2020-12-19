// +build !js

package websocket

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/libp2p/go-libp2p-core/sec/insecure"
	mplex "github.com/libp2p/go-libp2p-mplex"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	ma "github.com/multiformats/go-multiaddr"
)

var (
	wasmBrowserTestBin     = "wasmbrowsertest"
	wasmBrowserTestDir     = filepath.Join("tools", "bin")
	wasmBrowserTestPackage = "github.com/agnivade/wasmbrowsertest"
)

// TestInBrowser is a harness that allows us to use `go test` in order to run
// WebAssembly tests in a headless browser.
func TestInBrowser(t *testing.T) {
	// ensure we have the right tools.
	err := os.MkdirAll(wasmBrowserTestDir, 0755)

	t.Logf("building %s", wasmBrowserTestPackage)
	if err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}

	cmd := exec.Command(
		"go", "build",
		"-o", wasmBrowserTestBin,
		"github.com/agnivade/wasmbrowsertest",
	)
	cmd.Dir = wasmBrowserTestDir
	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("starting server")

	// Start a transport which the browser peer will dial.
	serverDoneSignal := make(chan struct{})
	go func() {
		defer func() {
			close(serverDoneSignal)
		}()
		tpt := New(&tptu.Upgrader{
			Secure: insecure.New("serverPeer"),
			Muxer:  new(mplex.Transport),
		})
		addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5555/ws")
		if err != nil {
			t.Fatal("SERVER:", err)
		}
		listener, err := tpt.Listen(addr)
		if err != nil {
			t.Fatal("SERVER:", err)
		}
		conn, err := listener.Accept()
		if err != nil {
			t.Fatal("SERVER:", err)
		}
		defer conn.Close()
		stream, err := conn.OpenStream(context.Background())
		if err != nil {
			t.Fatal("SERVER: could not open stream:", err)
		}
		defer stream.Close()
		buf := bufio.NewReader(stream)
		if _, err := stream.Write([]byte("ping\n")); err != nil {
			t.Fatal("SERVER:", err)
		}
		msg, err := buf.ReadString('\n')
		if err != nil {
			t.Fatal("SERVER: could not read pong message:" + err.Error())
		}
		expected := "pong\n"
		if msg != expected {
			t.Fatalf("SERVER: Received wrong message. Expected %q but got %q", expected, msg)
		}
	}()

	t.Log("starting browser")

	cmd = exec.Command(
		"go", "test", "-v",
		"-exec", filepath.Join(wasmBrowserTestDir, wasmBrowserTestBin),
		"-run", "TestInBrowser",
		".",
	)
	cmd.Env = append(os.Environ(), []string{"GOOS=js", "GOARCH=wasm"}...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		formattedOutput := "\t" + strings.Join(strings.Split(string(output), "\n"), "\n\t")
		t.Log("BROWSER OUTPUT:\n", formattedOutput)
		t.Fatal("BROWSER:", err)
	}

	<-serverDoneSignal
}
