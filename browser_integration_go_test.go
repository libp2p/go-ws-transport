// +build !js

package websocket

import (
	"bufio"
	"fmt"
	"sync"
	"testing"

	"github.com/libp2p/go-libp2p-core/sec/insecure"
	mplex "github.com/libp2p/go-libp2p-mplex"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	ma "github.com/multiformats/go-multiaddr"
)

const (
	testServerPort = ":9714"
)

// TestInBrowser is a harness that allows us to use `go test` in order to run
// WebAssembly tests in a headless browser.
func TestInBrowser(t *testing.T) {
	// Start a transport which the browser peer will dial.
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		tpt := New(&tptu.Upgrader{
			Secure: insecure.New("serverPeer"),
			Muxer:  new(mplex.Transport),
		})
		addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5555/ws")
		if err != nil {
			t.Fatal(err)
		}
		listener, err := tpt.Listen(addr)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("server started listener")
		conn, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		fmt.Println("server received conn")
		stream, err := conn.OpenStream()
		if err != nil {
			fmt.Println("Could not open stream:", err.Error())
			t.Fatal(err)
		}
		defer stream.Close()
		fmt.Println("server opened stream")
		buf := bufio.NewReader(stream)
		if _, err := stream.Write([]byte("ping\n")); err != nil {
			t.Fatal(err)
		}
		fmt.Println("server wrote message")
		msg, err := buf.ReadString('\n')
		if err != nil {
			t.Fatal("could not read pong message:" + err.Error())
		}
		fmt.Println("server received message")
		expected := "pong\n"
		if msg != expected {
			t.Fatalf("Received wrong message. Expected %q but got %q", expected, msg)
		}
	}()

	// fmt.Println("Starting headless browser tests...")
	// cmd := exec.Command("go", "test", "-exec", `"$GOPATH/bin/wasmbrowsertest"`, "-run", "TestInBrowser", ".")
	// cmd.Env = append(os.Environ(), []string{"GOOS=js", "GOARCH=wasm"}...)
	// if output, err := cmd.CombinedOutput(); err != nil {
	// 	t.Log(string(output))
	// 	t.Fatal(err)
	// }

	wg.Wait()
}
