package tcp

import (
	"net"

	network "github.com/panlibin/vgnet"
)

// Dialer 连接器
var Dialer dialer

type dialer struct {
}

func (d *dialer) Dial(addr string, cb func(network.Connection, error)) {
	if cb == nil {
		return
	}

	go func() {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			cb(nil, err)
			return
		}
		tcpConn := newConnection(conn)
		cb(tcpConn, nil)

		tcpConn.run()
	}()
}
