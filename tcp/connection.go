package tcp

import (
	"errors"
	"net"
	"sync"

	logger "github.com/panlibin/vglog"
	network "github.com/panlibin/vgnet"
)

const _DefaultWriteChannelSize = 64

const (
	statusInit int32 = iota
	statusRunning
	statusWaitForClose
	statusClosed
)

type connection struct {
	mtx       sync.Mutex
	conn      net.Conn
	writeChan chan interface{}
	agent     network.Agent
	wg        sync.WaitGroup
	status    int32
	lastErr   error
}

func newConnection(conn net.Conn) *connection {
	newConn := new(connection)
	newConn.conn = conn
	newConn.status = statusInit
	newConn.wg.Add(1)
	return newConn
}

// Accept 接受连接,设置agent
func (c *connection) Accept(agent network.Agent) {
	c.mtx.Lock()
	if c.status != statusInit {
		c.mtx.Unlock()
		return
	}
	if agent == nil {
		c.status = statusWaitForClose
	} else {
		c.agent = agent
		c.writeChan = make(chan interface{}, _DefaultWriteChannelSize)
		c.status = statusRunning
	}
	c.mtx.Unlock()
	c.wg.Done()
}

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

func (c *connection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

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

	c.conn.(*net.TCPConn).SetLinger(0)
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
		err = c.agent.DoRead(c.conn)
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
		err := c.agent.DoWrite(c.conn, msg)
		if err != nil {
			break
		}
	}

	c.Close(err)
	c.conn.(*net.TCPConn).SetLinger(0)
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
