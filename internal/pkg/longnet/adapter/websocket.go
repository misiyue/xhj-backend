package adapter

import (
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// WsAdapter WebsocketAddr 适配器
type WsAdapter struct {
	conn *websocket.Conn
}

var defaultUpGrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// NewUpGraderWithOrigins creates a new websocket.Upgrader with configurable CORS origins
func NewUpGraderWithOrigins(origins string) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			if origins == "*" {
				return true
			}

			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}

			// Split origins by comma and check if the request origin is allowed
			allowedOrigins := strings.Split(origins, ",")
			for _, allowed := range allowedOrigins {
				if strings.TrimSpace(allowed) == origin {
					return true
				}
			}

			return false
		},
	}
}

func NewWsAdapter(w http.ResponseWriter, r *http.Request, upgrader *websocket.Upgrader) (*WsAdapter, error) {
	var upgrade = upgrader
	if upgrade == nil {
		upgrade = &defaultUpGrader
	}

	conn, err := upgrade.Upgrade(w, r, w.Header())
	if err != nil {
		return nil, err
	}

	return &WsAdapter{
		conn: conn,
	}, nil
}

func (w *WsAdapter) Network() string {
	return NetworkWss
}

func (w *WsAdapter) Read() ([]byte, error) {
	_, content, err := w.conn.ReadMessage()
	return content, err
}

func (w *WsAdapter) Write(bytes []byte) error {
	return w.conn.WriteMessage(websocket.TextMessage, bytes)
}

func (w *WsAdapter) Close() error {
	_ = w.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(time.Second),
	)

	return w.conn.Close()
}

func (w *WsAdapter) SetCloseHandler(fn func(code int, text string) error) {
	w.conn.SetCloseHandler(fn)
}

// SetReadDeadline 设置读取超时时间
func (w *WsAdapter) SetReadDeadline(deadline time.Time) error {
	return w.conn.SetReadDeadline(deadline)
}

// SetWriteDeadline 设置写入超时时间
func (w *WsAdapter) SetWriteDeadline(deadline time.Time) error {
	return w.conn.SetWriteDeadline(deadline)
}
