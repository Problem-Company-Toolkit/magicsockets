# MagicSocket

MagicSocket is a reusable and efficient WebSockets library designed to simplify the management of WebSocket clients and the sending and receiving of messages in your projects.

## Why MagicSocket?

WebSockets play a crucial role in real-time applications by keeping users engaged and informed. MagicSocket was created to provide an easy-to-use and flexible solution for managing WebSocket clients and their messages, making it an ideal choice for any project that requires WebSockets.

## What Can MagicSocket Do?

MagicSocket offers the following features:

- Effortless management of WebSocket clients
- Simplified sending and receiving of messages
- Seamless integration with projects that require WebSockets
- User-friendly APIs

## Usage Examples

### Creating a MagicSocket Instance

To begin, create a new MagicSocket instance:

```go
import "github.com/problem-company-toolkit/magicsocket"

socket := magicsockets.New(magicsockets.SocketOpts{
    Port: 8080,
})
```

### Handling Client Connections

Register clients with MagicSocket using the `OnConnect` callback, which is called whenever a new client connects:

```go
socket := magicsockets.New(magicsockets.SocketOpts{
    Port: 8080,
    OnConnect: func(r *http.Request) (magicsockets.RegisterClientOpts, error) {
        // Perform any authentication or validation here.

        return magicsockets.RegisterClientOpts{
            Key: "client-key", // Unique key to identify the client
            Topics: []string{"topic1", "topic2"}, // Topics the client is interested in
        }, nil
    },
})
```

### Sending Messages

Send messages to clients by emitting events on specific topics:

```go
socket.Emit(magicsockets.EmitOpts{
    Rules: []magicsockets.EmitRule{
        {
            Topics: []string{"topic1"},
            Key:    "client-key",
        },
    },
}, []byte("Hello, users!"))
```

### Receiving Messages

Handle incoming messages from clients using the `OnIncoming` callback:

```go
socket := magicsockets.New(magicsockets.SocketOpts{
    Port: 8080,
    OnConnect: func(r *http.Request) (magicsockets.RegisterClientOpts, error) {
        // Perform any authentication or validation here.

        return magicsockets.RegisterClientOpts{
            Key: "client-key", // Unique key to identify the client
            Topics: []string{"topic1", "topic2"}, // Topics the client is interested in
            OnIncoming: func(messageType int, data []byte) error {
                // Handle the incoming message here.

                return nil
            },
        }, nil
    },
})
```