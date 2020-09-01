package websocket

import (
	"errors"
	"io"
	"net"
	"sync"

	"github.com/gorilla/websocket"
	logger "github.com/panlibin/vglog"
	network "github.com/panlibin/vgnet"
)

const (
	statusInit int32 = iota
	statusRunning
	statusWaitForClose
	statusClosed
)

type connection struct {
	mtx       sync.Mutex
	conn      *websocket.Conn
	writeChan chan interface{}
	agent     network.Agent
	wg        sync.WaitGroup
	status    int32
	lastErr   error
	writer    websocketWriter
}

func newConnection(conn *websocket.Conn) *connection {
	newConn := new(connection)
	newConn.conn = conn
	newConn.status = statusInit
	newConn.writer.c = conn
	newConn.wg.Add(1)
	return newConn
}

// Accept 接受连接,设置agent
func (c *connection) Accept(agent network.Agent, writeChanSize int) {
	c.mtx.Lock()
	if c.status != statusInit {
		c.mtx.Unlock()
		return
	}
	if agent == nil {
		c.status = statusWaitForClose
	} else {
		c.agent = agent
		c.writeChan = make(chan interface{}, writeChanSize)
		c.status = statusRunning
	}
	c.mtx.Unlock()
	c.wg.Done()
}

// Write 发送
func (c *connection) Write(msg interface{}) error {
	if msg == nil {
		return errors.New("can not write a nil message")
	}

	c.mtx.Lock()

	if c.status != statusRunning {
		c.mtx.Unlock()
		return errors.New("connection is closed")
	}

	if len(c.writeChan) == cap(c.writeChan) {
		logger.Warning("connection write channel full")
	}

	c.mtx.Unlock()

	c.writeChan <- msg

	return nil
}

// LocalAddr 获取本地地址
func (c *connection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// LocalAddr 获取远程地址
func (c *connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// Close 关闭连接
func (c *connection) Close(err error) {
	c.mtx.Lock()
	if c.status == statusWaitForClose || c.status == statusClosed {
		c.mtx.Unlock()
		return
	}
	status := c.status
	c.status = statusWaitForClose
	c.lastErr = err
	c.mtx.Unlock()

	if status == statusInit {
		c.wg.Done()
	} else if status == statusRunning {
		c.writeChan <- nil
	}
}

func (c *connection) doClose() {
	c.mtx.Lock()
	if c.status != statusWaitForClose {
		c.mtx.Unlock()
		return
	}

	c.status = statusClosed

	if c.writeChan != nil {
		close(c.writeChan)
	}

	c.conn.Close()

	agent := c.agent
	c.agent = nil

	c.mtx.Unlock()

	if agent != nil {
		agent.OnClose(c.lastErr)
	}
}

func (c *connection) readLoop() {
	var err error
	for {
		var r io.Reader
		_, r, err = c.conn.NextReader()
		if err != nil {
			break
		}

		err = c.agent.DoRead(r)
		if err != nil {
			break
		}
	}

	c.Close(err)
	c.wg.Done()
}

func (c *connection) writeLoop() {
	var err error
	for msg := range c.writeChan {
		if msg == nil {
			break
		}

		err := c.agent.DoWrite(c.writer, msg)
		if err != nil {
			break
		}
	}

	c.Close(err)
	c.conn.Close()
	c.wg.Done()
}

func (c *connection) run() {
	c.wg.Wait()
	if c.status != statusRunning {
		c.lastErr = errors.New("connection accept fail")
		c.doClose()
		return
	}
	c.wg.Add(2)
	go c.writeLoop()
	go c.readLoop()
	c.wg.Wait()

	c.doClose()
}

type websocketWriter struct {
	c *websocket.Conn
}

func (wsw websocketWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	err = wsw.c.WriteMessage(websocket.BinaryMessage, p)
	return
}
