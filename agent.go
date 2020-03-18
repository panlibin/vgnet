package network

import "io"

// Agent 消息处理接口
type Agent interface {
	DoRead(io.Reader) error
	DoWrite(io.Writer, interface{}) error

	OnClose(error)
}
