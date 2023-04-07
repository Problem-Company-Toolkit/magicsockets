package magicsocket

import (
	"errors"
	"net/http"

	"github.com/gorilla/websocket"
)

type clientConn struct {
	conn   *websocket.Conn
	key    string
	topics []string

	onIncoming   func(messageType int, data []byte) error
	onOutgoing   func(messageType int, data []byte) error
	onPing       func() error
	onDisconnect func() error
}

type RegisterClientOpts struct {
	Key          string
	Topics       []string
	OnIncoming   func(messageType int, data []byte) error
	OnOutgoing   func(messageType int, data []byte) error
	OnPing       func() error
	OnDisconnect func() error
}

type ClientConn interface {
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
	GetKey() string
	GetTopics() []string
}

func (ms *magicSocket) registerClient(w http.ResponseWriter, r *http.Request, opts RegisterClientOpts) error {
	ms.Lock()
	defer ms.Unlock()

	if _, ok := ms.clients[opts.Key]; ok {
		return errors.New("client already registered")
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow any origin
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	client := clientConn{
		conn:         conn,
		key:          opts.Key,
		topics:       opts.Topics,
		onIncoming:   opts.OnIncoming,
		onOutgoing:   opts.OnOutgoing,
		onPing:       opts.OnPing,
		onDisconnect: opts.OnDisconnect,
	}

	ms.clients[opts.Key] = client

	go ms.startOutgoingMessagesChannel(opts.Key, opts)
	go ms.startIncomingMessagesChannel(opts.Key, opts)

	return nil
}

func (cc *clientConn) GetKey() string {
	return cc.key
}

func (cc *clientConn) GetTopics() []string {
	return cc.topics
}

func (cc *clientConn) WriteMessage(messageType int, data []byte) error {
	return cc.conn.WriteMessage(messageType, data)
}

func (cc *clientConn) ReadMessage() (messageType int, p []byte, err error) {
	return cc.conn.ReadMessage()
}
