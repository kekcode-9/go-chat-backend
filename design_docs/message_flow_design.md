After [[The actual database design]]
1. What was hub before now becomes `WsConManager` who keeps a map of `userId`/`deviceId` to `ws` Client instance (the client has the `readpump`, `writepump` and the actual `ws` connection). Allows client registering, `unregistering` and writes messages to a client's send channel
# The registries in redis
1. device connection registry --> which device is connected to which backend server

# `Backends` connecting to Redis:
1. Each backend server gets an unique Id when it starts
2. Each backend subscribes to the same redis instance but in a channel of its own
3. ==The channel name is same as the backend server id==

# Devices connecting to backend server
1. When a user's device makes a websocket connection request the `WsConManager` registers it with a new Client and informs redis
2. Redis then adds the `device_id` or `user_id` to `backend_id` entry to its device connection registry

# The message flow:
1. A message becomes available on the `websocket`. The Client's `readpump` reads the message (which has info about the conversation_id as well) and hands the message over to an `IncomingMssgHandler` along with message content, sender `uuid` and conversation_id 
2. The `IncomingMessageHandler` starts a database transaction, atomically allocates the next sequence number for the conversation, inserts the message, updates `next_sequence_no`, commits the transaction.
3. The `IncomingMessageHandler` queries all participants to that particular conversation and finds a list of user ids and for each user id it finds a list of device ids linked to those users. It also finds any of the senders other device ids since the message also needs to be sent there. ==Use a table join between user and devices tables==
4. Then the `IncomingMessageHandler` (or we could have another function responsible for this part) queries the device connection registry asking which backend is connected to each of the participant's devices
5. For each of the backend devices the handler determines the channels they are connected to by the backend server id
6. The `IncomingMessageHandler` then publishes the message to each channel connecting to the target backends. Before publishing the target backends are grouped like: device A, X, and T are to backend 112, like so. Then one message is published per backend. The message contains the list of participant device ids, the sender user id, the conversation id and the complete message detail. 
	- `message_id`
	- `conversation_id`
	- `sequence_no`
	- sender information
	- target device IDs
	- message payload
7. When a backend subscribed to redis sees a message event in one of the channels it is listening to, it checks the list of target devices and finds which websocket clients each of these devices are linked to by consulting its `WsConManager` and then writes the message to the send channel to each of these websocket Clients
8. The client's `writepump` sees this message in the send channel and writes it to the `websocket`

# Resolving user to device
When a user writes a message to their websocket connection they will mention their user_id (uuid), the device id (uuid), the conversation id (uuid). The `IncomingMessageHandler` finds the devices of all the participant users of that conversations (from the `conversation_participants` and the `devices` tables). The message will be passed to the other participant devices as well as the other devices of the same user. When broadcasting a message over redis, only device_id are taken into account. not user id.