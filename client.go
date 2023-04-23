package magicsockets

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type client struct {
	mutex  *sync.Mutex
	logger *zap.Logger

	id  string
	key string

	onIncoming   func(messageType int, data []byte) error
	onOutgoing   func(messageType int, data []byte) error
	onPing       func() error
	onDisconnect func() error

	getServer func() *magicSocket

	topics []string
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
	GetID() string
	GetKey() string
	GetTopics() []string

	Close() error

	UpdateKey(string) error
	SetTopics([]string)

	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
}

func (ms *magicSocket) registerClient(w http.ResponseWriter, r *http.Request, opts RegisterClientOpts) error {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

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

	clientID := uuid.New().String()
	logger := ms.logger.With(zap.String("Client ID", clientID))
	client := client{
		mutex:        &sync.Mutex{},
		logger:       logger,
		id:           clientID,
		key:          opts.Key,
		topics:       opts.Topics,
		onIncoming:   opts.OnIncoming,
		onOutgoing:   opts.OnOutgoing,
		onPing:       opts.OnPing,
		onDisconnect: opts.OnDisconnect,
		getServer: func() *magicSocket {
			return ms
		},
	}

	ms.connections[clientID] = conn
	ms.clients[clientID] = &client

	go ms.startIncomingMessagesChannel(client.id, opts)

	return nil
}

func (cc *client) UpdateKey(newKey string) error {
	ms := cc.getServer()
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	_, ok := ms.clientKeys[newKey]
	if ok {
		return fmt.Errorf("key %s already in use", newKey)
	}

	delete(ms.clientKeys, cc.key)
	cc.key = newKey
	ms.clientKeys[newKey] = cc.id

	return nil
}

func (cc *client) SetTopics(topics []string) {
	ms := cc.getServer()
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	cc.topics = topics
}

func (cc *client) GetKey() string {
	return cc.key
}

func (cc *client) GetID() string {
	return cc.id
}

func (cc *client) GetTopics() []string {
	return cc.topics
}

func (cc *client) Close() error {
	ms := cc.getServer()

	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	cc.logger.Debug("Closing client connection")

	if ms.connections[cc.id] != nil {
		err := ms.connections[cc.id].Close()
		if err != nil {
			cc.logger.Error("Failed to close client connection", zap.Error(err))
			return err
		}
	}

	if cc.onDisconnect != nil {
		if err := cc.onDisconnect(); err != nil {
			cc.logger.Error("Failed to process onDisconnect", zap.Error(err))
		}
	}

	delete(ms.connections, cc.id)
	delete(ms.clients, cc.id)
	delete(ms.clientKeys, cc.key)

	cc = nil
	return nil
}

// Sends a message to the client.
func (cc *client) WriteMessage(messageType int, data []byte) error {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	defer func() {
		val := recover()
		if val != nil {
			valString := fmt.Sprintf("%+v", val)
			if strings.Contains(valString, "invalid memory address or nil pointer dereference") {
				cc.logger.Debug("Client connection closed abruptly. Removing client...")
				cc.Close()

				return
			}
		}

	}()

	ms := cc.getServer()
	conn := ms.connections[cc.id]
	if conn == nil {
		return fmt.Errorf("connection already closed")
	}
	return ms.connections[cc.id].WriteMessage(messageType, data)
}

// Consumes a message sent from the client.
func (cc *client) ReadMessage() (messageType int, p []byte, err error) {
	s := cc.getServer()
	return s.connections[cc.id].ReadMessage()
}
