package magicsockets

import (
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type EmitOpts struct {
	Rules []EmitRule
}

type EmitRule struct {
	Keys   []string
	Topics []string
}

func (ms *magicSocket) Emit(opts EmitOpts, message []byte) {
	ms.Lock()
	defer ms.Unlock()

	targets := ms.clients

	for _, rule := range opts.Rules {
		for k, v := range targets {
			matchesKeys := rule.Keys == nil || len(rule.Keys) == 0 || contains(rule.Keys, v.key)
			matchesTopics := rule.Topics == nil || len(rule.Topics) == 0 || containsAll(rule.Topics, v.topics)

			if !matchesKeys || !matchesTopics {
				delete(targets, k)
			}
		}
	}

	for k := range targets {
		client := ms.clients[k]
		go func() {
			messageType := websocket.TextMessage

			logger := ms.logger.With(
				zap.String("Client Key", client.key),
				zap.String("Message ID", uuid.New().String()),
				zap.Int("Message Type", messageType),
			)

			logger.Info("Emitting message")

			err := client.conn.WriteMessage(messageType, message)
			if err != nil {
				logger.Error("Send message to client error", zap.Error(err))
			}
			if client.onOutgoing != nil {
				if err := client.onOutgoing(messageType, message); err != nil {
					logger.Error("onOutgoing error", zap.Error(err))
				}
			}
		}()
	}
}
