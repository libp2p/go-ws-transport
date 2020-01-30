// +build js,wasm

package websocket

// WebsocketConfig is the configuration type for the Websocket transport.
//
// In JS and WASM build targets, it is empty for now.
type WebsocketConfig struct {
}

// DefaultWebsocketConfig returns a default Websocket transport configuration.
//
// In JS and WASM build targets, it is empty for now.
func DefaultWebsocketConfig() *WebsocketConfig {
	return &WebsocketConfig{}
}
