package websocket

import (
	"log"
	"testing"
	"time"

	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
)

var _ = log.Println

func TestMultiaddrParsing(t *testing.T) {
	addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5555/ws")
	if err != nil {
		t.Fatal(err)
	}

	_, err = parseMultiaddr(addr)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWebsocketListen(t *testing.T) {
	zero, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/0/ws")
	if err != nil {
		t.Fatal(err)
	}

	tpt := &WebsocketTransport{}
	l, err := tpt.Listen(zero)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		d, _ := tpt.Dialer(nil)
		c, err := d.Dial(l.Multiaddr())
		if err != nil {
			t.Error(err)
			return
		}

		c.Write([]byte("HELLO WORLD!"))
		time.Sleep(time.Second)
		c.Close()
	}()

	c, err := l.Accept()
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 32)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("READ: %s", buf[:n])
	c.Close()
	l.Close()
	time.Sleep(time.Second)
}
