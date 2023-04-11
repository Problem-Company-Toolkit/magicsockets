package magicsockets

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type (
	onConnectFunc = func(*http.Request) (RegisterClientOpts, error)
)

type MagicSocket interface {
	Emit(opts EmitOpts, message []byte)

	GetClients() map[string]ClientConn

	SetOnConnect(onConnectFunc)

	Start() error

	// No-op if hasn't been started yet.
	Stop() error

	GetPort() int
}

type magicSocket struct {
	logger *zap.Logger
	sync.Mutex

	connections map[string]*websocket.Conn
	clients     map[string]*client

	clientKeys map[string]string

	onConnect onConnectFunc

	server *http.Server
	port   int
}

type MagicSocketOpts struct {
	OnConnect onConnectFunc
	Port      int
}

func New(opts MagicSocketOpts) MagicSocket {
	logger, _ := zap.NewDevelopment()
	logger = logger.With(
		zap.String("MagicSocket ID", uuid.NewString()),
	)

	return &magicSocket{
		logger:  logger,
		clients: make(map[string]*client),

		clientKeys: make(map[string]string),

		connections: make(map[string]*websocket.Conn),
		onConnect:   opts.OnConnect,
		port:        opts.Port,
	}
}

func (ms *magicSocket) connectionStillExists(clientID string) bool {
	client := ms.clients[clientID]
	websocketConn := ms.connections[clientID]

	return client != nil && websocketConn != nil
}

func (ms *magicSocket) SetOnConnect(onConnect onConnectFunc) {
	ms.onConnect = onConnect
}

func (ms *magicSocket) GetPort() int {
	return ms.port
}

func (ms *magicSocket) GetClients() map[string]ClientConn {
	ms.Lock()
	defer ms.Unlock()

	clients := make(map[string]ClientConn)
	for _, v := range ms.clients {
		clients[v.key] = v
	}
	return clients
}

func (ms *magicSocket) startIncomingMessagesChannel(clientID string, opts RegisterClientOpts) {
	// Make sure the client won't be updated while we're getting its reference.
	ms.Lock()
	client := ms.clients[clientID]
	ms.Unlock()

	logger := ms.logger.With(zap.String("Client ID", clientID))

	defer func() {
		recover()
		if ms.connectionStillExists(clientID) {
			ms.clients[clientID].Close()
		}
	}()

	logger.Debug("Starting listening")
	for ms.connectionStillExists(clientID) {
		messageType, message, err := client.ReadMessage()
		logger.Debug("Received incoming message")
		if err != nil {
			if strings.Contains(err.Error(), "close 1006") {
				// ms.logger.Debug(
				// 	"Connection closed unexpectedly",
				// 	zap.Int("Code", 1006),
				// 	zap.Error(err),
				// )
			} else if strings.Contains(err.Error(), "reset by peer") ||
				strings.Contains(err.Error(), "use of closed network connection") {
				client.Close()
			} else {
				logger.Error(
					"Write Error",
					zap.String("Message", string(message)),
					zap.Error(err),
				)
			}
			break
		}

		if client.onIncoming != nil {
			if err := client.onIncoming(messageType, message); err != nil {
				logger.Error("client onIncoming error", zap.Error(err))
			}
		}
	}

	logger.Debug("Terminating listening")
}

// Blocks as long as the server is listening.
func (ms *magicSocket) Start() error {
	ms.server = &http.Server{Addr: fmt.Sprintf(":%d", ms.GetPort())}

	mux := http.NewServeMux() // Create a new ServeMux for each
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if ms.onConnect != nil {
			opts, err := ms.onConnect(r)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
				return
			}
			if err := ms.registerClient(w, r, opts); err != nil {
				ms.logger.Panic(err.Error())
			}
		} else {
			if err := ms.registerClient(w, r, RegisterClientOpts{
				Key: uuid.NewString(),
			}); err != nil {
				ms.logger.Panic(err.Error())
			}
		}
	})
	ms.server.Handler = mux

	ms.logger.Info("Starting MagicSocket websockets server", zap.Int("Port", ms.GetPort()))
	err := ms.server.ListenAndServe()
	// We consider this to be a successful exit.
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (ms *magicSocket) Stop() error {
	if ms == nil || ms.server == nil {
		return nil
	}
	ms.logger.Debug("Closing MagicSockets server and stopping all connections")

	// We must add items to a slice
	// Instead of directly closing them via the map
	// Because you can't iterate over a map and make changes to it at the same time.
	ms.Lock()
	clientsToClose := []*client{}
	for i := range ms.clients {
		clientsToClose = append(clientsToClose, ms.clients[i])
	}
	ms.Unlock()
	for i := range clientsToClose {
		clientsToClose[i].Close()
	}

	return ms.server.Close()
}
