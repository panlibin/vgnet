package websocket

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	logger "github.com/panlibin/vglog"
	network "github.com/panlibin/vgnet"
)

type wsHandler struct {
	upgrader        *websocket.Upgrader
	newConnCallback func(network.Connection)
}

func (wsh *wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	conn, err := wsh.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Errorf("websocket upgrade error: %v", err)
		return
	}
	conn.SetReadLimit(100000)
	wsConn := newConnection(conn)
	wsh.newConnCallback(wsConn)

	wsConn.run()
}

// Listener websocket监听器
type Listener struct {
	Addr            string
	HTTPTimeout     time.Duration
	CertFile        string
	KeyFile         string
	NewConnCallback func(network.Connection)

	listener net.Listener
	handler  *wsHandler
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

	if l.HTTPTimeout <= 0 {
		l.HTTPTimeout = time.Second * 10
	}

	if l.CertFile != "" || l.KeyFile != "" {
		config := &tls.Config{}
		config.NextProtos = []string{"http/1.1"}

		var err error
		config.Certificates = make([]tls.Certificate, 1)
		config.Certificates[0], err = tls.LoadX509KeyPair(l.CertFile, l.KeyFile)
		if err != nil {
			return err
		}

		ln = tls.NewListener(ln, config)
	}

	l.listener = ln

	upgrader := new(websocket.Upgrader)
	upgrader.HandshakeTimeout = l.HTTPTimeout
	upgrader.CheckOrigin = func(_ *http.Request) bool { return true }
	handler := new(wsHandler)
	handler.upgrader = upgrader
	handler.newConnCallback = l.NewConnCallback
	l.handler = handler

	httpServer := new(http.Server)
	httpServer.Addr = l.Addr
	httpServer.Handler = handler
	httpServer.ReadTimeout = l.HTTPTimeout
	httpServer.WriteTimeout = l.HTTPTimeout
	httpServer.MaxHeaderBytes = 1024

	go httpServer.Serve(ln)

	return nil
}

// Stop 停止监听
func (l *Listener) Stop() {
	l.listener.Close()
}
