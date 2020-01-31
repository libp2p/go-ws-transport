// +build !js

package websocket

import (
	"crypto/tls"
	"net/http"

	"github.com/gorilla/websocket"
)

// WebsocketConfig is the configuration type for the Websocket transport.
type WebsocketConfig struct {
	WebsocketUpgrader *websocket.Upgrader
	TLSClientConfig   *tls.Config
}

// DefaultWebsocketConfig returns a default Websocket transport configuration.
//
// The default configuration on native environments returns an unconditional
// true for CORS requests, and has an empty TLS client configuration.
func DefaultWebsocketConfig() *WebsocketConfig {
	return &WebsocketConfig{
		WebsocketUpgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		TLSClientConfig: &tls.Config{},
	}
}

// WebsocketUpgrader sets a custom gorilla/websocket upgrader on the WebSocket
// transport.
func WebsocketUpgrader(u *websocket.Upgrader) Option {
	return func(cfg *WebsocketConfig) error {
		cfg.WebsocketUpgrader = u
		return nil
	}
}

// TLSClientConfig sets a TLS client configuration on the WebSocket Dialer. Only
// relevant for non-browser usages.
//
// Some useful use cases include setting InsecureSkipVerify to `true`, or
// setting user-defined trusted CA certificates.
func TLSClientConfig(c *tls.Config) Option {
	return func(cfg *WebsocketConfig) error {
		cfg.TLSClientConfig = c
		return nil
	}
}
