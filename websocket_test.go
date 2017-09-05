package websocket

import (
	"bytes"
	"io/ioutil"
	"testing"
	"testing/iotest"

	ma "github.com/multiformats/go-multiaddr"
)

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
	defer l.Close()

	msg := []byte("HELLO WORLD")

	go func() {
		d, _ := tpt.Dialer(nil)
		c, err := d.Dial(l.Multiaddr())
		if err != nil {
			t.Error(err)
			return
		}

		c.Write(msg)
		c.Close()
	}()

	c, err := l.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	obr := iotest.OneByteReader(c)

	out, err := ioutil.ReadAll(obr)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, msg) {
		t.Fatal("got wrong message", out, msg)
	}
}
