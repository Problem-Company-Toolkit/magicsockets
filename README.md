# MagicSockets

MagicSockets is a simple yet powerful WebSocket library for Go that makes it easy to manage client connections, emit messages based on rules, and handle incoming and outgoing messages.

## Features

- Easy management of WebSocket client connections.
- Key-based and topic-based client identification.
- Emit messages to clients using rules for keys and topics.
- Receive incoming messages and send outgoing messages through custom handlers.
- Register and update client information easily.

## Installation

To install MagicSockets, use the following command:

```
go get -u github.com/problem-company-toolkit/magicsockets
```

## Usage

### Starting the MagicSocket server

```
import (
	"github.com/problem-company-toolkit/magicsockets"
)

func main() {
	ms := magicsockets.New(magicsockets.MagicSocketOpts{
		Port: 8080,
	})
	ms.Start() // Blocking
}
```

### Connecting clients

To connect a client to the MagicSocket server, simply initiate a standard WebSocket connection, like so:

```
const socket = new WebSocket("ws://localhost:8080");
```

### Emitting messages to clients

The following will only send messages to clients that match all rules:

```
ms.Emit(magicsockets.EmitOpts{
	Rules: []magicsockets.EmitRule{
		{
			Keys: []string{"clientKey1", "clientKey2"},
		},
		{
			Topics: []string{"topic1", "topic2"},
		},
	},
}, []byte("Hello, clients!"))
```

### Registering a websocket connection

```
ms.SetOnConnect(func(r *http.Request) (magicsockets.RegisterClientOpts, error) {
	return magicsockets.RegisterClientOpts{
		Key:    "clientKey",
		Topics: []string{"topic1", "topic2"},
		OnIncoming: func(messageType int, data []byte) error {
			fmt.Printf("Received message: %s\n", string(data))
			return nil
		},
		OnOutgoing: func(messageType int, data []byte) error {
			fmt.Printf("Sending message: %s\n", string(data))
			return nil
		},
	}, nil
})
```

### Updating client information

MagicSockets support updating the abstract "client" that is connecting to the server.

This way, you can have a single WebSocket connection, but change its identifier and topics according to your application's logic, without making any changes to the underlying WebSocket connection.

```
client := ms.GetClients()["clientKey"]

// Update client key
client.UpdateKey("newClientKey")

// Update client topics
client.SetTopics([]string{"newTopic1", "newTopic2"})
```

Example of where you might want to use this:

- Establishing one WebSocket connection per user. By manipulating the client key, you could set it to the user's ID once you receive an event that identifies the user.
- You can further reuse the same connection if the user participates in other activities or stops participating within the same domain by simply updating the topics.

## Running tests

To run the tests, execute the following command in the terminal:

```
ginkgo
```