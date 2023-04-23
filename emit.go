package magicsockets

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type EmitOpts struct {
	Rules []EmitRule
}

type EmitRule struct {
	OnlyKeys    []string
	AnyOfTopics []string
}

func (ms *magicSocket) Emit(opts EmitOpts, message []byte) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	ms.logger.Debug(
		"Starting emit message processing",
		zap.String("Options", fmt.Sprintf("%s", opts)),
	)

	targets := make(map[string]*client)

	for _, rule := range opts.Rules {
		for clientID, client := range ms.clients {
			matchesKeys := rule.OnlyKeys == nil || contains(rule.OnlyKeys, client.key)

			// Auto approve if it's nil.
			matchesTopics := rule.AnyOfTopics == nil

			if !matchesTopics {
				match := false
				for _, query := range rule.AnyOfTopics {
					match = contains(client.topics, query)
					if match {
						break
					}
				}

				matchesTopics = match
			}

			if matchesKeys && matchesTopics {
				targets[clientID] = client
			}
		}
	}

	ms.logger.Debug(
		"Will emit to targets",
		zap.String("Targets", fmt.Sprintf("%v", targets)),
	)

	for _, client := range targets {
		messageType := websocket.TextMessage

		logger := client.logger.With(
			zap.String("Client Key", client.key),
			zap.String("Message ID", uuid.New().String()),
			zap.Int("Message Type", messageType),
		)

		logger.Info("Emitting message")

		err := client.WriteMessage(messageType, message)
		if err != nil {
			logger.Error("Send message to client error", zap.Error(err))
			return
		}
		if client.onOutgoing != nil {
			if err := client.onOutgoing(messageType, message); err != nil {
				logger.Error("onOutgoing error", zap.Error(err))
				return
			}
		}
	}
}
