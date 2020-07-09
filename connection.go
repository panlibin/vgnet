package network

import (
	"net"
)

// Connection socket
type Connection interface {
	Accept(Agent, int)
	Write(interface{}) error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	Close(error)
}
