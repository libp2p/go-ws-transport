package websocket

import (
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"syscall/js"
	"time"
)

const (
	webSocketStateConnecting = 0
	webSocketStateOpen       = 1
	webSocketStateClosing    = 2
	webSocketStateClosed     = 3
)

var errIsClosed = errors.New("connection is closed")

// Conn implements net.Conn interface for WebSockets in js/wasm.
type Conn struct {
	js.Value
	messageHandler *js.Func
	closeHandler   *js.Func
	mut            sync.Mutex
	incomingData   chan []byte
	currData       *bytes.Buffer
	closeSignal    chan struct{}
}

func (c *Conn) Read(b []byte) (int, error) {
	c.mut.Lock()
	defer c.mut.Unlock()
	if err := c.checkOpen(); err != nil {
		return 0, io.EOF
	}

	if c.currData == nil {
		select {
		case data := <-c.incomingData:
			c.currData = bytes.NewBuffer(data)
		case <-c.closeSignal:
			return 0, io.EOF
		}
	}

	n, err := c.currData.Read(b)
	if err == io.EOF {
		c.currData = nil
		return n, nil
	}
	return n, err
}

func (c *Conn) checkOpen() error {
	state := c.Get("readyState").Int()
	switch state {
	case webSocketStateClosed, webSocketStateClosing:
		return errIsClosed
	}
	return nil
}

func (c *Conn) Write(b []byte) (n int, err error) {
	// c.mut.Lock()
	// defer c.mut.Unlock()
	if err := c.checkOpen(); err != nil {
		return 0, err
	}
	c.Call("send", string(b))
	return len(b), nil
}

// Close closes the connection. Only the first call to Close will receive the
// close error, subsequent and concurrent calls will return nil.
// This method is thread-safe.
func (c *Conn) Close() error {
	// c.mut.Lock()
	// defer c.mut.Unlock()
	c.Call("close")
	if c.messageHandler != nil {
		c.Call("removeEventListener", "message", *c.messageHandler)
		c.messageHandler.Release()
	}
	if c.closeHandler != nil {
		c.Call("removeEventListener", "close", *c.closeHandler)
		c.closeHandler.Release()
	}
	return nil
}

func (c *Conn) LocalAddr() net.Addr {
	// TODO(albrow): is there some way to get the local address?
	return NewAddr("0.0.0.0:0")
}

func (c *Conn) RemoteAddr() net.Addr {
	rawURL := c.Get("url").String()
	withoutPrefix := strings.TrimPrefix(rawURL, "ws://")
	withoutSuffix := strings.TrimSuffix(withoutPrefix, "/")
	return NewAddr(withoutSuffix)
}

func (c *Conn) SetDeadline(t time.Time) error {
	// Not supported
	return nil
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	// Not supported
	return nil
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	// Not supported
	return nil
}

// NewConn creates a Conn given a regular js/wasm WebSocket Conn.
func NewConn(raw js.Value) *Conn {
	conn := &Conn{
		Value:        raw,
		incomingData: make(chan []byte),
		closeSignal:  make(chan struct{}),
	}
	conn.setUpHandlers()
	return conn
}

func (c *Conn) setUpHandlers() {
	// c.mut.Lock()
	// defer c.mut.Unlock()
	if c.messageHandler != nil {
		// Message handlers already created. Nothing to do.
		return
	}
	messageHandler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		go func() {
			// TODO(albrow): Currently we assume data is of type Blob. Really we
			// should check binaryType and then decode accordingly.
			blob := args[0].Get("data")
			text := readBlob(blob)
			c.incomingData <- []byte(text)
		}()
		return nil
	})
	c.messageHandler = &messageHandler
	c.Call("addEventListener", "message", messageHandler)

	closeHandler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		close(c.closeSignal)
		return nil
	})
	c.closeHandler = &closeHandler
	c.Call("addEventListener", "close", closeHandler)
}

func (c *Conn) waitForOpen() error {
	openSignal := make(chan struct{})
	handler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		close(openSignal)
		return nil
	})
	defer func() {
		c.Call("removeEventListener", "open", handler)
	}()
	defer handler.Release()
	c.Call("addEventListener", "open", handler)
	<-openSignal
	return nil
}

func readBlob(blob js.Value) string {
	// const reader = new FileReader();
	// reader.addEventListener('loadend', (e) => {
	//   const text = e.srcElement.result;
	//   console.log(text);
	// });
	// reader.readAsText(blb);
	reader := js.Global().Get("FileReader").New()
	textChan := make(chan string)
	loadEndFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		go func() {
			text := args[0].Get("srcElement").Get("result").String()
			textChan <- text
		}()
		return nil
	})
	defer loadEndFunc.Release()
	reader.Call("addEventListener", "loadend", loadEndFunc)
	reader.Call("readAsText", blob)
	return <-textChan
}
