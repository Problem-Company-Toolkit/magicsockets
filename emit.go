package magicsockets

import (
	"encoding/json"
	"log"

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

func (ms *magicSocket) Emit(opts EmitOpts, message interface{}) {
	ms.Lock()
	defer ms.Unlock()

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Println("JSON marshal error:", err)
		return
	}

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
			err := client.conn.WriteMessage(messageType, messageBytes)
			if err != nil {
				logger.Error("Send message to client error", zap.Error(err))
			}
			if client.onOutgoing != nil {
				if err := client.onOutgoing(messageType, messageBytes); err != nil {
					logger.Error("onOutgoing error", zap.Error(err))
				}
			}
		}()
	}
}
