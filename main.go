package magicsocket

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	logger, _ = zap.NewDevelopment()
)

type (
	onConnectFunc = func(*http.Request) (RegisterClientOpts, error)
)

type MagicSocket interface {
	Emit(opts EmitOpts, message interface{})
	GetClients() map[string]ClientConn

	SetOnConnect(onConnectFunc)

	Start() error

	// No-op if hasn't been started yet.
	Stop() error

	GetPort() int
}

type magicSocket struct {
	sync.Mutex
	clients map[string]clientConn

	onConnect onConnectFunc

	server *http.Server
	port   int
}

type MagicSocketOpts struct {
	OnConnect onConnectFunc
	Port      int
}

func New(opts MagicSocketOpts) MagicSocket {
	return &magicSocket{
		clients:   make(map[string]clientConn),
		onConnect: opts.OnConnect,
		port:      opts.Port,
	}
}

func (ms *magicSocket) connectionStillExists(key string) bool {
	client, ok := ms.clients[key]
	return ok && client.conn != nil
}

func (ms *magicSocket) SetOnConnect(onConnect onConnectFunc) {
	ms.onConnect = onConnect
}

func (ms *magicSocket) GetPort() int {
	return ms.port
}

func (ms *magicSocket) GetClients() map[string]ClientConn {
	clients := make(map[string]ClientConn)
	for k, v := range ms.clients {
		clients[k] = &v
	}
	return clients
}

func (ms *magicSocket) startOutgoingMessagesChannel(key string, opts RegisterClientOpts) {
	defer func() {
		recover()
		ms.Lock()
		delete(ms.clients, key)
		ms.Unlock()
		if opts.OnDisconnect != nil {
			opts.OnDisconnect()
		}
	}()

	conn := ms.clients[key].conn
	for ms.connectionStillExists(key) {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if strings.Contains(err.Error(), "close 1006") {
				// logger.Debug(
				// 	"Connection closed unexpectedly",
				// 	zap.Int("Code", 1006),
				// 	zap.Error(err),
				// )
			} else if strings.Contains(err.Error(), "use of closed network connection") {

			} else if strings.Contains(err.Error(), "reset by peer") {

			} else {
				logger.Error("Read Error", zap.Error(err))
			}
			break
		}
	}
}

func (ms *magicSocket) startIncomingMessagesChannel(key string, opts RegisterClientOpts) {
	defer func() {
		recover()
		if ms.connectionStillExists(key) {
			ms.clients[key].conn.Close()
		}
	}()

	conn := ms.clients[key].conn
	client := ms.clients[key]
	for ms.connectionStillExists(key) {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if strings.Contains(err.Error(), "close 1006") {
				// logger.Debug(
				// 	"Connection closed unexpectedly",
				// 	zap.Int("Code", 1006),
				// 	zap.Error(err),
				// )
			} else if strings.Contains(err.Error(), "reset by peer") {

			} else {
				logger.Error("Write Error", zap.Error(err))
			}
			break
		}

		if client.onIncoming != nil {
			if err := client.onIncoming(messageType, message); err != nil {
				logger.Error("client onIncoming error", zap.Error(err))
			}
		}

		err = conn.WriteMessage(messageType, message)
		if err != nil {
			log.Println("WriteMessage error:", err)
			break
		}
	}
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
				logger.Panic(err.Error())
			}
		} else {
			if err := ms.registerClient(w, r, RegisterClientOpts{
				Key: uuid.NewString(),
			}); err != nil {
				logger.Panic(err.Error())
			}
		}
	})
	ms.server.Handler = mux

	logger.Info("Starting MagicSocket websockets server", zap.Int("Port", ms.GetPort()))
	err := ms.server.ListenAndServe()
	// We consider this to be a successful exit.
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (ms *magicSocket) Stop() error {
	if ms.server == nil {
		return nil
	}

	return ms.server.Close()
}
