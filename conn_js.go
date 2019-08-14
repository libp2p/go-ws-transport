package websocket

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
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

const incomingDataBufferSize = 100

var errConnectionClosed = errors.New("connection is closed")

// Conn implements net.Conn interface for WebSockets in js/wasm.
type Conn struct {
	js.Value
	messageHandler *js.Func
	closeHandler   *js.Func
	mut            sync.Mutex
	incomingData   chan []byte
	currDataMut    sync.RWMutex
	currData       bytes.Buffer
	closeSignal    chan struct{}
	dataSignal     chan struct{}
}

// NewConn creates a Conn given a regular js/wasm WebSocket Conn.
func NewConn(raw js.Value) *Conn {
	conn := &Conn{
		Value:        raw,
		incomingData: make(chan []byte, incomingDataBufferSize),
		closeSignal:  make(chan struct{}),
		dataSignal:   make(chan struct{}),
	}
	conn.setUpHandlers()
	go func() {
		// TODO(albrow): Handle error appropriately
		err := conn.readLoop()
		if err != nil {
			panic(err)
		}
	}()
	return conn
}

func (c *Conn) Read(b []byte) (int, error) {
	if err := c.checkOpen(); err != nil {
		return 0, io.EOF
	}

	for {
		c.currDataMut.RLock()
		n, err := c.currData.Read(b)
		c.currDataMut.RUnlock()
		if err != nil && err != io.EOF {
			// Return any unexpected errors immediately.
			return n, err
		} else if n == 0 || err == io.EOF {
			// There is no data ready to be read. Wait for more data or for the
			// connection to be closed.
			select {
			case <-c.dataSignal:
				continue
			case <-c.closeSignal:
				return 0, io.EOF
			}
		} else {
			return n, err
		}
	}
}

// checkOpen returns an error if the connection is not open. Otherwise, it
// returns nil.
func (c *Conn) checkOpen() error {
	state := c.Get("readyState").Int()
	switch state {
	case webSocketStateClosed, webSocketStateClosing:
		return errConnectionClosed
	}
	return nil
}

func (c *Conn) Write(b []byte) (n int, err error) {
	if err := c.checkOpen(); err != nil {
		return 0, err
	}
	uint8Array := js.Global().Get("Uint8Array").New(len(b))
	for i := 0; i < len(b); i++ {
		uint8Array.SetIndex(i, b[i])
	}
	c.Call("send", uint8Array.Get("buffer"))
	return len(b), nil
}

// Close closes the connection. Only the first call to Close will receive the
// close error, subsequent and concurrent calls will return nil.
// This method is thread-safe.
func (c *Conn) Close() error {
	c.mut.Lock()
	defer c.mut.Unlock()
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
	return NewAddr("0.0.0.0:0")
}

func (c *Conn) RemoteAddr() net.Addr {
	rawURL := c.Get("url").String()
	withoutPrefix := strings.TrimPrefix(rawURL, "ws://")
	withoutSuffix := strings.TrimSuffix(withoutPrefix, "/")
	return NewAddr(withoutSuffix)
}

func (c *Conn) SetDeadline(t time.Time) error {
	return os.ErrNoDeadline
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return os.ErrNoDeadline
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return os.ErrNoDeadline
}

func (c *Conn) setUpHandlers() {
	c.mut.Lock()
	defer c.mut.Unlock()
	if c.messageHandler != nil {
		// Message handlers already created. Nothing to do.
		return
	}
	messageHandler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		go func() {
			// TODO(albrow): Currently we assume data is of type Blob. Really we
			// should check binaryType and then decode accordingly.
			blob := args[0].Get("data")
			data := readBlob(blob)
			c.incomingData <- data
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

// readLoop continuosly reads from the c.incoming data channel and writes to the
// current data buffer.
func (c *Conn) readLoop() error {
	for data := range c.incomingData {
		c.currDataMut.Lock()
		_, err := c.currData.Write(data)
		if err != nil {
			c.currDataMut.Unlock()
			return err
		}
		c.currDataMut.Unlock()
		c.dataSignal <- struct{}{}
	}
	return nil
}

func (c *Conn) waitForOpen() error {
	openSignal := make(chan struct{})
	handler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		close(openSignal)
		return nil
	})
	defer c.Call("removeEventListener", "open", handler)
	defer handler.Release()
	c.Call("addEventListener", "open", handler)
	<-openSignal
	return nil
}

// readBlob converts a JavaScript Blob into a slice of bytes. It uses the
// FileReader API under the hood.
func readBlob(blob js.Value) []byte {
	reader := js.Global().Get("FileReader").New()
	dataChan := make(chan []byte)
	loadEndFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		// event.result is of type ArrayBuffer. We can convert it to a JavaScript
		// Uint8Array, which is directly translatable to the Go type []byte.
		buffer := reader.Get("result")
		view := js.Global().Get("Uint8Array").New(buffer)
		dataLen := view.Get("length").Int()
		data := make([]byte, dataLen)
		for i := 0; i < dataLen; i++ {
			data[i] = byte(view.Index(i).Int())
		}
		dataChan <- data
		return nil
	})
	defer loadEndFunc.Release()
	reader.Call("addEventListener", "loadend", loadEndFunc)
	reader.Call("readAsArrayBuffer", blob)
	return <-dataChan
}
