package websocket

import (
	"github.com/gorilla/websocket"
	network "github.com/panlibin/vgnet"
)

// Dialer 连接器
var Dialer dialer

type dialer struct {
	wsDialer *websocket.Dialer
}

func init() {
	Dialer.wsDialer = new(websocket.Dialer)
}

func (d *dialer) Dial(addr string, cb func(network.Connection, error)) {
	if cb == nil {
		return
	}

	go func() {
		conn, _, err := d.wsDialer.Dial(addr, nil)
		if err != nil {
			cb(nil, err)
			return
		}
		wsConn := newConnection(conn)
		cb(wsConn, nil)
		wsConn.run()
	}()
}
