After [[Message flow with Redis]] and [[The actual database design]]
# Goal:
- Setup redis pub/sub for multiple backends and test out passing of websocket messages between the sender and receiver channels

# Scope:
- No database exists yet. So no tables that repository methods can query or update to.
- Right now we will test with hardcoded user id (like "user_alice", "user_bob"), hardcoded device ids like "alice_device_android", "bob_device_macbook", "bob_device_iphone" like this.
- We will create at least two backends - one to which alice's devices are connected and another to which bob's devices are connected. 
- We will create a hardcoded `DeviceCon` (device id to backend server id map) registry in redis.
- The `WsClient` will not have anything hardcoded. It will be the proper client with proper `ReadPump` and `WritePump`
- We will handle `ws` client registering and unregistering properly. Converting http to websocket will also be done properly.
- Only database related things are hardcoded for now.

# First:
## Create models (structs) for communication among structs and all:
- A model called `OutgoingMessage` is mandatory which will have the following:
	- `message_id`
	- `conversation_id`
	- `sequence_no`
	- sender user id (`uuid`)
	- sender name
	- target device Id (`uuid`)
	- message payload
	- message `timestamp`

## Build the `webscoket` Client - name the struct `WsClient`:
- It will be a struct with `UserId` of type `uuid`, `DeviceId` of type `uuid`, reference to `WsConManager`, reference to the `websocket` connection
- It will have `ReadPump` and `WritePump`

## Build the `WsConManager`
- It will have a `deviceWsCon` registry which is a map from device id to `WsClient` instance
- It will handle the registering, `unregistering` of clients taking register and `unregister` requests from two channels `Register` and `Unregister`
- It will have a `RouteMessage` channel where messages are queued with each message having all the relevant sender, receiver and message info and the target device's id. It will read messages from `RouteMessage` and write it to the target device's client's send channel. `RouteMessage` is a channel for `OutgoingMessage` objects.

## Build the `MessageService`
- It is a struct exposing an `InMssgHandler` and `OutMssgHandler` method
- This struct also has a reference to the shared `redis` instance (shared between `backends`). It publishes to this `redis` instance and also subscribes to this `redis` instance, listening on the channel pertaining to its own `backend` (every `backend` has this `MessageService` struct)
- The `InMssgHandler` is called with message content, timestamp, sender `uuid`, sender device `uuid` and conversation `uuid`. 
	- It calls the proper repository methods to find the target device ids (other devices of the sender and all devices of every participant of this conversation).
	- It finds out the `backend` groups of the target devices from `redis`'s `DeviceCon` registry (device id to `backend` server id map). 
	- It publishes the message to every channel on which one of the target `backend`s are listening
- The `OutMssgHandler` is called by `MessageService` when a message is posted on `redis` on the subscribed channel and the `OutMssgHandler` writes this message to the `RouteMessage` channel of the `WsConManager`

---

# Folder structure
For your first iteration, I'd avoid organizing by technical layers (`handlers/`, `services/`, `repositories/`). Since you already know your major domains (`websocket`, `message`, `redis`), it's better to organize **by feature/module**. That will make extracting a module into a microservice much easier later.

For example, instead of:

```text
internal/
    handlers/
    services/
    repositories/
    models/
```

prefer:

```text
internal/
    websocket/
    messaging/
    redisbus/
    registry/
```

Each module owns everything related to itself.

---

# Folder structure
```
chat-backend/
│
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   │
│   ├── config/
│   │   └── config.go
│   │
│   ├── models/
│   │   ├── chat_message.go
│   │   └── websocket.go
│   │
│   ├── websocket/
│   │   ├── client.go
│   │   ├── manager.go
│   │   └── handler.go
│   │
│   ├── message/
│   │   ├── service.go
│   │   ├── publisher.go
│   │   └── subscriber.go
│   │
│   ├── repository/
│   │   └── mock_repository.go
│   │
│   ├── redis/
│   │   └── redis.go
│   │
│   └── server/
│       └── websocket.go
│
├── go.mod
└── go.sum
```

## `cmd/server/main.go`
Responsibilities:
- load config
- create Redis client
- create repository
- create `WsConManager`
- create `MessageService`
- wire dependencies together
- start HTTP server

## `internal/config/config.go`
Contains:
```go
type Config struct {
    BackendID string

    RedisAddr string

    HttpPort string
}
```

This can later be read from the environment.
## `internal/models`
Contains on `DTO`'s (Data Transfer Object).

`chat_message.go` The
## `internal/websocket`
Everything related to `websocket`.
### `client.go`
Contains the `WsClient` struct with the `ReadPump` and `WritePump` methods.
### `manager.go`
Contains the `WsConManager` struct. Handles:
- `Register`
- `Unregister`
- `RouteMessage`
- The `deviceWsCon` registry.
- `Run()`
### `handler.go`
Contains:
- `ServeWs()` 
Responsibilities:
- Upgrades `http`
- Creates `WsClient`
- Register client
- Start `ReadPump`
- Start `WritePump`
## `internal/messages`
Owns all message business logic.
### `service.go`
Contains `MessageService` struct, `InMssgHandler` and `OutMssgHandler`.
It talks to `repository`, `redis` and `WsConManager`.
### `publisher.go`
Wrapper around `Redis`. Contains `Publish()`. Nothing else.
### `subscriber.go`
Starts:
```go
SUBSCRIBE backend:<id>
```
Keeps reading `Redis`.
Whenever a message arrives, calls `OutMssgHandler()`. This file is basically one [[Coroutine#`goroutine`|`goroutine`]].
## `internal/repository`
Since there is no DB yet
```
mock_repository.go
```

Contains
```
type Repository struct
```

Methods like
```
FindConversationParticipants()

FindDevicesForUsers()

...
```

Everything is hardcoded.
Later this package becomes
```
message_repository.go

conversation_repository.go

device_repository.go
```

without changing MessageService.

## `internal/redis`
Contains Redis initialization.
```
redis.go
```

Creates
```
redis.Client
```

No business logic.
## `internal/server`
Owns HTTP
For now only
```
websocket.go
```
Contains
```
RegisterRoutes(...)
```
Later
```
message.go

conversation.go

user.go
```
when REST APIs appear.
