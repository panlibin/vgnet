package tcp

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	logger "github.com/panlibin/vglog"
	network "github.com/panlibin/vgnet"
)

// Listener Tcp监听器
type Listener struct {
	Addr            string
	NewConnCallback func(network.Connection)

	listener net.Listener
	wg       sync.WaitGroup
	closed   int32
}

// Start 开始监听
func (l *Listener) Start() error {
	ln, err := net.Listen("tcp", l.Addr)
	if err != nil {
		return err
	}

	if l.NewConnCallback == nil {
		return errors.New("new connection callback is nil")
	}

	l.listener = ln

	go l.accept()

	return nil
}

func (l *Listener) accept() {
	l.wg.Add(1)
	defer l.wg.Done()

	for {
		conn, err := l.listener.Accept()

		if atomic.LoadInt32(&l.closed) != 0 {
			if conn != nil {
				conn.Close()
			}
			break
		}

		if err != nil {
			logger.Warning(err)
			time.Sleep(time.Second)
			continue
		}

		go l.newConnection(conn)
	}
}

func (l *Listener) newConnection(conn net.Conn) {
	tcpConn := newConnection(conn)
	l.NewConnCallback(tcpConn)

	tcpConn.run()
}

// Stop 停止监听
func (l *Listener) Stop() {
	if !atomic.CompareAndSwapInt32(&l.closed, 0, 1) {
		return
	}
	l.listener.Close()
	l.wg.Wait()
}
