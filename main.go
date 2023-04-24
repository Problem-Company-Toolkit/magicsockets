package magicsockets

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
	mutex  *sync.Mutex

	isRunning bool

	connections map[string]*websocket.Conn
	clients     map[string]*client

	// Key is the Client Key, which is mutable.
	// Value is Client ID, which should be immutable ever since it is created.
	clientKeys map[string]string

	onConnect onConnectFunc

	gracePeriod time.Duration

	server *http.Server
	port   int
}

type MagicSocketOpts struct {
	OnConnect onConnectFunc
	Port      int

	LoggerOpts LoggerOpts

	GracePeriod time.Duration
}

type LoggerOpts struct {
	Logger   *zap.Logger
	Encoding string
	LogLevel *zapcore.Level
}

const (
	DEFAULT_LOG_LEVEL    = zapcore.WarnLevel
	DEFAULT_LOG_ENCODING = "console"
)

const (
	_DEFAULT_GRACE_PERIOD = time.Second * 4
)

func New(opts MagicSocketOpts) MagicSocket {
	var err error

	loggerOpts := opts.LoggerOpts
	logger := loggerOpts.Logger
	if logger == nil {
		zapLevel := zap.NewAtomicLevel()

		level := func() zapcore.Level {
			if opts.LoggerOpts.LogLevel != nil {
				return *opts.LoggerOpts.LogLevel
			}

			envLevel := os.Getenv("MAGICSOCKETS_LOG_LEVEL")
			if envLevel == "" {
				return DEFAULT_LOG_LEVEL
			}

			switch strings.ToLower(envLevel) {
			case "debug":
				return zapcore.DebugLevel
			case "info":
				return zapcore.InfoLevel
			case "warn":
				return zapcore.WarnLevel
			case "fatal":
				return zapcore.FatalLevel
			case "panic":
				return zapcore.PanicLevel
			case "dpanic":
				return zapcore.DPanicLevel
			default:
				fmt.Printf(
					"\nWARNING: Invalid Log Level passed to MagicSockets via environment variable: %s. Will use default log level: %s\n",
					envLevel,
					DEFAULT_LOG_LEVEL.String(),
				)
				return DEFAULT_LOG_LEVEL
			}
		}()

		zapLevel.SetLevel(level)

		encoding := func() string {
			if loggerOpts.Encoding == "" {
				return DEFAULT_LOG_ENCODING
			}

			return loggerOpts.Encoding
		}()

		config := zap.Config{
			Level:             zapLevel,
			Development:       false,
			DisableCaller:     true,
			DisableStacktrace: true,
			OutputPaths:       []string{"stdout"},
			ErrorOutputPaths:  []string{"stderr"},
			Encoding:          encoding,
			EncoderConfig: zapcore.EncoderConfig{
				TimeKey:        "timestamp",
				LevelKey:       "level",
				MessageKey:     "message",
				CallerKey:      "caller",
				EncodeTime:     zapcore.ISO8601TimeEncoder,
				EncodeLevel:    zapcore.LowercaseLevelEncoder,
				EncodeDuration: zapcore.StringDurationEncoder,
				EncodeCaller:   zapcore.ShortCallerEncoder,
			},
		}
		logger, err = config.Build()
		if err != nil {
			panic(fmt.Errorf("failed to build logger configurations for magicsockets: %s", err.Error()))
		}
	}
	logger = logger.With(
		zap.String("MagicSocket ID", uuid.NewString()),
	)

	gracePeriod := opts.GracePeriod
	if gracePeriod == 0 {
		gracePeriod = _DEFAULT_GRACE_PERIOD
	}

	return &magicSocket{
		mutex:   &sync.Mutex{},
		logger:  logger,
		clients: make(map[string]*client),

		clientKeys: make(map[string]string),

		gracePeriod: gracePeriod,

		connections: make(map[string]*websocket.Conn),
		onConnect:   opts.OnConnect,
		port:        opts.Port,
	}
}

func (ms *magicSocket) SetOnConnect(onConnect onConnectFunc) {
	ms.onConnect = onConnect
}

func (ms *magicSocket) GetPort() int {
	return ms.port
}

func (ms *magicSocket) GetClients() map[string]ClientConn {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	clients := make(map[string]ClientConn)
	for _, v := range ms.clients {
		clients[v.key] = v
	}
	return clients
}

func (ms *magicSocket) startIncomingMessagesChannel(clientID string, opts RegisterClientOpts) {
	// Make sure the client won't be updated while we're getting its reference.
	client := func() *client {
		ms.mutex.Lock()
		defer ms.mutex.Unlock()
		return ms.clients[clientID]
	}()

	logger := ms.logger.With(zap.String("Client ID", clientID))

	defer func() {
		recover()
		if client != nil {
			client.Close()
		}
	}()

	logger.Debug("Starting listening")
	for client != nil {
		messageType, message, err := client.ReadMessage()
		logger.Debug("Received incoming message", zap.String("Message", string(message)))
		if err != nil {
			logger.Error("Error receiving message", zap.Error(err))

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

	ms.isRunning = true

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
	ms.mutex.Lock()

	ms.isRunning = false

	clientsToClose := []*client{}
	for i := range ms.clients {
		clientsToClose = append(clientsToClose, ms.clients[i])
	}
	ms.mutex.Unlock()
	for i := range clientsToClose {
		clientsToClose[i].Close()
	}

	return ms.server.Close()
}
