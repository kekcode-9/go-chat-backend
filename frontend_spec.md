# Frontend Client Specification

This document captures the current backend contract and the frontend flows needed for a browser client for this chat application.

## Backend Overview

- Default HTTP base URL: `http://localhost:8080`
- Expected frontend origin for CORS: `http://localhost:3000`
- Auth model: short-lived JWT access token in frontend storage plus refresh token in the `X-Refresh-Token` response/request header.
- Access token lifetime: 15 minutes.
- Refresh token/session lifetime: 30 days.
- Refresh token header:
  - Header name: `X-Refresh-Token`
  - Returned by signup, login, and refresh responses.
  - Sent by the frontend only to `POST /auth/refresh/`.
- All protected REST calls require:
  - `Authorization: Bearer <access_token>`
  - `Content-Type: application/json` when sending a JSON body
- The React Native client must store the refresh token securely and send it as the raw `X-Refresh-Token` header value for refresh only.

## Important Integration Gaps

1. Browser WebSocket authentication needs a backend change or proxy support.
   - The backend protects `/ws/` with the same middleware that requires an `Authorization` header.
   - Native browser `WebSocket` does not allow setting custom headers.
   - A browser frontend cannot directly call `new WebSocket("ws://localhost:8080/ws/")` with the required bearer header.
   - Options:
     - Change backend to accept `access_token` in the WebSocket query string.
     - Change backend to accept an auth cookie for WebSocket.
     - Use a frontend/dev proxy that injects `Authorization`.
     - Use a non-browser WebSocket client where headers are supported.

2. Signup and login return session IDs in the response body.
   - Both return `{ access_token, user_id, device_id }`.
   - The JWT claims also include `user_id` and `device_id`, but the frontend should use the response body for normal session setup.

3. WebSocket acknowledgements/errors are not JSON in the current implementation.
   - The code defines JSON shapes for `message_ack` and `error`, but currently writes only a plain string:
     - On message ack: the `client_message_id` string.
     - On validation errors: the human-readable error message string.
   - Incoming delivered messages from other devices/users are JSON.

4. Some registered routes are stubs.
   - `POST /conversations/participant`
   - `DELETE /conversations/participant`
   - `DELETE /conversations`
   - `DELETE /conversations/leave`
   - These currently do not implement useful behavior and should not be used by the frontend yet.

5. User lookup documents `username`, but only email lookup is implemented.
   - `GET /users/?email=...` works.
   - `GET /users/?username=...` returns `400` unless `email` is also provided.

6. Unknown-device login handling may currently surface as `500`.
   - The handler has a `401 unknown device` branch.
   - The repository path for an unknown device currently returns the raw no-row database error before the service converts it to `ErrUnknownDevice`.
   - Frontend should still clear stale `device_id` on either `401 unknown device` or a repeatable login failure after sending a stored `device_id`.

7. Message fetch/send paths do not currently verify conversation membership.
   - The endpoints are protected by access token, but `GET /conversations/{id}/messages` and WebSocket message send do not check that the caller is an active participant before accessing the conversation.
   - A production frontend should still only expose conversations returned by `GET /conversations`, but this should be fixed on the backend.

## Frontend App Areas

### Public/Auth Screens

- Signup
- Login
- Auth session restore/loading

### Authenticated Screens

- Conversation list
- Direct conversation creation by user search
- Chat thread
- Participant/presence view
- Logout
- Optional block user action

## Data Models for the Frontend

Use string types for UUID and timestamp values.

```ts
type AuthSession = {
  accessToken: string;
  accessTokenExpiresAt?: string;
  userId: string;
  deviceId: string;
};

type User = {
  id: string;
  user_name: string;
  email: string;
};

type ConversationParticipant = {
  participant_id: string;
  participant_name: string;
};

type ParticipantWithPresence = {
  participant_id: string;
  participant_name: string;
  is_online: boolean;
};

type ConversationSummary = {
  conversation_id: string;
  participants: ConversationParticipant[];
};

type ChatMessage = {
  message_id: string;
  conversation_id: string;
  sequence_no: number;
  sender_user_id: string;
  sender_name: string;
  payload: string;
  timestamp: string;
};

type PendingMessage = {
  client_message_id: string;
  conversation_id: string;
  payload: string;
  created_at: string;
  status: "pending" | "acked" | "failed";
};
```

## Local Data to Store

Persist these across reloads:

- `access_token`
  - Needed for protected REST calls.
  - Can be kept in memory for better security, but then page refresh requires `POST /auth/refresh/`.
  - If persisted, prefer a single auth/session store and clear it on logout or refresh failure.
- `device_id`
  - Send on future logins from the same device.
  - Obtained from signup/login response bodies, or decoded JWT after refresh.
- `user_id`
  - Obtained from signup/login response bodies, or decoded JWT after refresh.
- Conversation summaries
  - Keyed by `conversation_id`.
- Messages
  - Keyed by `conversation_id`, ordered by `sequence_no`.
- Per-conversation sync cursors:
  - `latest_sequence_no_by_conversation`
  - `oldest_sequence_no_by_conversation`
  - `last_read_sequence_no_by_conversation`
- Pending outbound messages
  - Keyed by `client_message_id`.
  - Required for optimistic UI and retry/idempotency.

Store the refresh token in secure client storage. Do not put it in ordinary app state or logs.

## API Calls

### Signup

`POST /auth/signup/`

Request:

```json
{
  "user_name": "Ruu",
  "email": "ruu@example.com",
  "password": "password"
}
```

Success `201`:

```json
{
  "access_token": "jwt",
  "user_id": "uuid",
  "device_id": "uuid"
}
```

Frontend behavior:

- Read the refresh token from the `X-Refresh-Token` response header and store it securely.
- Store `access_token`.
- Store `user_id` and `device_id` from the response body.
- Load conversations.
- Open WebSocket only after the WebSocket auth gap is addressed.

Errors:

- `400 invalid request body`
- `409 email already exists.`
- `500 internal server error`

### Login

`POST /auth/login/`

First login from a device:

```json
{
  "email": "ruu@example.com",
  "password": "password"
}
```

Subsequent login from known device:

```json
{
  "email": "ruu@example.com",
  "password": "password",
  "device_id": "uuid"
}
```

Success `200`:

```json
{
  "access_token": "jwt",
  "user_id": "uuid",
  "device_id": "uuid"
}
```

Frontend behavior:

- Include stored `device_id` when available.
- If the backend rejects a stored `device_id`, clear local `device_id` and retry login once without it.
- Store returned `access_token`, `user_id`, and `device_id`.

Errors:

- `400 invalid request body`
- `401 invalid credentials`
- `401 unknown device`
- `404 Not existing user.`
- `500 internal server error`, including possible current unknown-device repository behavior

### Refresh Access Token

`POST /auth/refresh/`

Request:

- No JSON body.
- Must include `X-Refresh-Token: <refresh_token>`.

Success `200`:

```json
{
  "access_token": "jwt"
}
```

Frontend behavior:

- Call on app boot if no usable access token exists.
- Call when a protected REST request returns `401 invalid token`.
- Rotate stored access token on success.
- Decode the new JWT and update `user_id`/`device_id`.
- On `401`, clear auth state and route to login.

Errors:

- `401 refresh token header not found`
- `401 Invalid refresh token.`
- `401 Session expired. Please Log in.`
- `401 Suspicious token reuse.`
- `500 internal server error`

### Logout

`POST /auth/logout/`

Headers:

- `Authorization: Bearer <access_token>`

Success:

- `204 No Content`

Frontend behavior:

- Close WebSocket.
- Clear access token, user id, cached session state, and pending messages.
- Keep or clear `device_id` depending on desired UX. Keeping it allows future known-device login.

Errors:

- `401 missing Authorization header`
- `401 invalid token`
- `500 action failed`

### Search User

`GET /users/?email=<email>`

Headers:

- `Authorization: Bearer <access_token>`

Success `200`:

```json
{
  "id": "uuid",
  "user_name": "Ruu",
  "email": "ruu@example.com"
}
```

Frontend behavior:

- Use for finding a user before creating a direct conversation.
- Do not rely on username search yet.

Errors:

- `400 either email or username must be provided`
- `404 user not found`
- `401 missing or invalid access token`
- `500 internal server error`

Security note: current backend code does not verify active conversation membership before returning messages.

### Block User

`POST /users/block/`

Request:

```json
{
  "blocked_user_id": "uuid"
}
```

Success:

- `204 No Content`

Frontend behavior:

- Treat as an optional safety/moderation action.
- There is no unblock/list-blocked endpoint currently.

### List Conversations

`GET /conversations`

Success `200`:

```json
{
  "conversations": [
    {
      "conversation_id": "uuid",
      "participants": [
        {
          "participant_id": "uuid",
          "participant_name": "Ruu"
        }
      ]
    }
  ]
}
```

Frontend behavior:

- Load after login/signup/session restore.
- Derive direct conversation display name from the participant that is not the current user.
- The response does not include last message, unread count, conversation title, avatar, pinned, archived, muted, or last read sequence.

### Create Conversation

`POST /conversations`

Request:

```json
{
  "other_user_id": "uuid",
  "type": "direct"
}
```

Notes:

- `type` is optional and defaults to `direct`.
- Accepted values are `direct` and `group`, but group creation currently still only adds the requester and one other user.

Success `201`:

```json
{
  "conversation_id": "uuid"
}
```

Frontend behavior:

- Search user by email.
- Create direct conversation with `other_user_id`.
- On success, navigate to the chat thread and load messages.
- On `409`, the backend does not return the existing conversation id, so the frontend should refresh the conversation list and find the matching direct conversation locally.

Errors:

- `400 cannot start conversation with yourself`
- `400 invalid conversation type`
- `409 direct conversation already exists between the two users`
- `500 internal server error`

### Get Conversation Participants with Presence

`GET /conversations/{conversation_id}/participants`

Success `200`:

```json
{
  "participants": [
    {
      "participant_id": "uuid",
      "participant_name": "Ruu",
      "is_online": true
    }
  ]
}
```

Frontend behavior:

- Use when opening a conversation or showing conversation details.
- The current response excludes the caller from the participant list.
- Presence is based on Redis key TTL refreshed by active WebSocket pong handling.

Errors:

- `400 invalid id in path`
- `403 the user is not an active participant of the conversation`
- `500 internal server error`

### List Messages

`GET /conversations/{conversation_id}/messages`

Query params:

- `limit`: positive integer, defaults to `100`
- `before_seq`: return messages older than this sequence number
- `after_seq`: return messages newer than this sequence number

Rules:

- Do not send `before_seq` and `after_seq` together.
- Messages are returned in ascending `sequence_no`.

Initial/latest load:

`GET /conversations/{id}/messages?limit=100`

Load older:

`GET /conversations/{id}/messages?before_seq=<oldest_sequence_no>&limit=50`

Catch up:

`GET /conversations/{id}/messages?after_seq=<latest_sequence_no>&limit=100`

Success `200`:

```json
{
  "conversation_id": "uuid",
  "limit": "00000000-0000-0000-0000-000000000000",
  "messages": [
    {
      "message_id": "uuid",
      "conversation_id": "uuid",
      "sequence_no": 1,
      "sender_user_id": "uuid",
      "sender_name": "Ruu",
      "payload": "hello",
      "timestamp": "2026-09-02T10:00:00Z"
    }
  ]
}
```

Note: the response model currently has `limit` typed as UUID by mistake and the handler does not set it. Frontend should ignore `limit`.

Frontend behavior:

- Store messages by `conversation_id`.
- De-duplicate by `message_id` and by `sequence_no`.
- Track `oldest_sequence_no` and `latest_sequence_no`.
- Use `before_seq` for scrollback pagination.
- Use `after_seq` after reconnect, app resume, or failed WebSocket periods.

Errors:

- `400 invalid conversation_id`
- `400 invalid limit`
- `400 invalid before_seq`
- `400 invalid after_seq`
- `400 before_seq and after_seq cannot both be present`
- `401 missing or invalid access token`
- `500 internal server error`

### Submit Read Receipt Over REST

`POST /conversations/{conversation_id}/messages/read-receipt`

Request:

```json
{
  "message_id": "uuid",
  "sequence_no": 123
}
```

Success:

- `204 No Content`

Frontend behavior:

- Use when messages were loaded by REST or when WebSocket is unavailable.
- Only send for the highest visible/read message in a conversation.
- Do not send lower sequence numbers after higher ones; backend ignores lower updates.

Errors:

- `400 invalid conversation_id`
- `400 invalid request body`
- `400 message_id is required`
- `400 sequence_no must be positive`
- `401 missing auth claims`
- `500 internal server error`

## WebSocket Protocol

Endpoint:

- `GET /ws/`
- Current backend requires `Authorization: Bearer <access_token>` during upgrade.

### Client to Server: Send Message

```json
{
  "type": "message",
  "conversation_id": "uuid",
  "payload": "hello",
  "client_message_id": "uuid"
}
```

Frontend behavior:

- Generate `client_message_id` on the client before sending.
- Add message to UI as `pending`.
- Keep the same `client_message_id` if retrying the same message.
- Do not send empty payloads.
- Maximum incoming WebSocket message size on backend is 64 KiB.

Current ack behavior:

- On success, backend sends a text frame containing only the `client_message_id`.
- Treat that matching pending message as `acked`, but the server-generated `message_id` and `sequence_no` are not returned to the sender's originating device.
- The message is broadcast to the sender's other devices and recipient devices, not echoed to the originating device.

Recommended backend ack shape for future:

```json
{
  "type": "message_ack",
  "client_message_id": "uuid",
  "message_id": "uuid",
  "conversation_id": "uuid",
  "sequence_no": 123,
  "timestamp": "2026-09-02T10:00:00Z"
}
```

### Server to Client: Delivered Message

```json
{
  "message_id": "uuid",
  "conversation_id": "uuid",
  "sequence_no": 123,
  "sender_user_id": "uuid",
  "sender_name": "Ruu",
  "payload": "hello",
  "timestamp": "2026-09-02T10:00:00Z"
}
```

Frontend behavior:

- Insert into the matching conversation.
- De-duplicate by `message_id` and `sequence_no`.
- Update `latest_sequence_no`.
- If the active thread is open and the message is visible, send a read receipt.

### Client to Server: Read Receipt

```json
{
  "type": "read_receipt",
  "conversation_id": "uuid",
  "read_message_id": "uuid",
  "read_message_seq_no": 123
}
```

Frontend behavior:

- Send when the user has viewed messages up to a sequence number.
- Debounce/batch by conversation.
- Send only the highest read sequence number.

### Current Error Frames

The backend currently sends plain text error messages rather than JSON. Known cases:

- `missing payload`
- `missing conversation_id`
- `missing client_message_id`
- `message already exists`

The frontend should tolerate both plain text and JSON until the backend is normalized.

## Main Frontend Flows

### 1. App Boot / Session Restore

1. Read local auth session.
2. If an access token exists and is not expired, use it.
3. Otherwise call `POST /auth/refresh/` with `X-Refresh-Token`.
4. On refresh success, store the new access token and decoded claims.
5. Load `GET /conversations`.
6. For each active/open conversation, load latest messages.
7. Open WebSocket when authentication support is available.
8. On WebSocket reconnect, call `GET /conversations/{id}/messages?after_seq=<latest_sequence_no>` for active conversations.

### 2. Signup

1. Submit user name, email, password to `POST /auth/signup/`.
2. Store returned access token.
3. Store `user_id` and `device_id` from the response body.
4. Initialize empty conversation/message stores.
5. Navigate to conversation list.

### 3. Login

1. Read locally stored `device_id`.
2. Submit email, password, and optional `device_id` to `POST /auth/login/`.
3. If the stored device id is rejected, clear device id and retry without it once.
4. Store returned access token and device id.
5. Store `user_id` from the response body.
6. Load conversations and connect WebSocket.

### 4. Conversation List

1. Call `GET /conversations`.
2. Store conversation summaries.
3. For direct conversations, display the other participant.
4. Optionally fetch latest messages for each conversation to build previews, since the list endpoint does not include them.

### 5. Start Direct Chat

1. Search by email with `GET /users/?email=...`.
2. Call `POST /conversations` with `other_user_id`.
3. On `201`, navigate to the returned conversation id.
4. On `409`, refresh `GET /conversations` and find the existing direct conversation that contains that user.

### 6. Open Chat Thread

1. Load participants with `GET /conversations/{id}/participants`.
2. Load latest messages with `GET /conversations/{id}/messages?limit=100`.
3. Render ascending by `sequence_no`.
4. Update local oldest/latest sequence cursors.
5. Send read receipt for the latest visible message.

### 7. Send Message

1. Generate `client_message_id`.
2. Add a pending message to local UI.
3. Send WebSocket JSON with `type: "message"`.
4. On current plain-text ack matching `client_message_id`, mark pending as acked.
5. Use REST `after_seq` catch-up later to reconcile server `message_id`, `sequence_no`, and timestamp for messages sent by this device.
6. If WebSocket is unavailable, keep message pending. There is no REST send-message endpoint currently.

### 8. Receive Message

1. Parse incoming WebSocket JSON.
2. Insert into message store if not already present.
3. Update conversation preview and latest sequence.
4. If the conversation is open/visible, send read receipt.
5. Otherwise mark conversation as locally unread.

### 9. Load Older Messages

1. Use the current `oldest_sequence_no`.
2. Call `GET /conversations/{id}/messages?before_seq=<oldest>&limit=50`.
3. Prepend returned messages.
4. Stop pagination when an empty array is returned.

### 10. Reconnect / Catch Up

1. Reopen WebSocket.
2. For each active conversation, call `GET /conversations/{id}/messages?after_seq=<latest_sequence_no>`.
3. Merge messages into local store.
4. Re-send read receipt for the highest visible/read message if needed.
5. Retry pending outbound messages with the same `client_message_id`.

### 11. Logout

1. Call `POST /auth/logout/`.
2. Close WebSocket.
3. Clear local auth/session state.
4. Clear cached conversations/messages if using a shared browser profile.
5. Route to login.

## Error Handling Requirements

- Wrap all protected REST calls with a single API client that:
  - Adds `Authorization`.
  - Reads and stores `X-Refresh-Token` from auth responses.
  - Sends `X-Refresh-Token` only to `POST /auth/refresh/`.
  - On `401 invalid token`, calls refresh once and retries the original request.
  - On refresh failure, clears auth state.
- Show specific user-facing messages for:
  - Email already exists.
  - Invalid credentials.
  - User not found.
  - Direct conversation already exists.
  - Cannot start conversation with yourself.
- Treat `500` and network failures as retryable/system errors.

## Storage Dump / Debug Export

For debugging and bug reports, the frontend should be able to dump a sanitized JSON snapshot of local client state:

```json
{
  "session": {
    "has_access_token": true,
    "user_id": "uuid",
    "device_id": "uuid"
  },
  "conversations": {},
  "messages_by_conversation": {},
  "cursors": {
    "latest_sequence_no_by_conversation": {},
    "oldest_sequence_no_by_conversation": {},
    "last_read_sequence_no_by_conversation": {}
  },
  "pending_messages": {}
}
```

Do not include:

- Raw access token
- Passwords
- Refresh token, which should not be accessible to JavaScript anyway

## Currently Unsupported Frontend Features

These exist in schema comments or placeholder models but are not fully implemented in the current API:

- REST message sending
- Group participant add/remove/leave/delete flows
- Conversation rename/title/avatar/description
- Typing indicators
- Message edit/delete
- Attachments
- Reactions
- Delivery receipts/ticks beyond last-read tracking
- Unblock/list blocked users
- Server-provided unread counts
- Server-provided conversation previews/last message
- Push notifications

## Recommended Backend Additions for a Production-Ready Frontend

- Browser-compatible WebSocket authentication.
- JSON WebSocket ack/error frames.
- Echo or ack full server message metadata back to the originating device.
- `GET /auth/me` to return the current user/device after session refresh.
- Return existing `conversation_id` on duplicate direct conversation creation.
- Add REST fallback for sending messages.
- Add unread count and last message fields to `GET /conversations`.
- Add implemented group-management endpoints or remove stub routes until ready.
