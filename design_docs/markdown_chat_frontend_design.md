# Chat and Markdown Chat Frontend Design

Last updated: 2026-10-23

Status: living frontend specification. Existing API contracts below are verified against the backend source. Markdown chat behavior is being designed and has not been implemented.

## 1. Purpose, Status, and Maintenance

Use this document to build the frontend for the current chat backend and to track the planned Markdown chat experience. Keep it updated whenever an API, response, socket frame, or product decision changes. Update the context summary, relevant flow, endpoint contract, capability matrix, and change history together.

Status labels:

- **Current:** implemented in the backend; the frontend may call it as documented.
- **Confirmed future behavior:** agreed product behavior; backend contracts may still be missing.
- **Recommended frontend behavior:** a proposed implementation choice, not an existing server guarantee.
- **Open:** requires a design decision before implementation.
- **Stub:** registered route with no implemented operation; do not expose it as functional.

The backend design companion is [markdown_chat_backend_design.md](markdown_chat_backend_design.md). The root [frontend_spec.md](../frontend_spec.md) is the earlier integration reference. This document is the maintained comprehensive frontend design; on contradictions, verify current source and update both references. Do not invent request schemas or endpoints for unfinished features and present them as current APIs.

## 2. Context Summary for Resuming Work

This is a Go chat backend with JWT access tokens, rotating refresh tokens, PostgreSQL persistence, Redis routing, and WebSockets. The frontend does not yet exist in this repository. No UI framework has been selected by this design.

Current features are signup, login, refresh, logout, exact-email user lookup, blocking, conversation creation/listing, active participant/presence lookup, sequence-paginated message history, message sending over WebSocket, and read receipts over REST or WebSocket. Several group-management routes are empty stubs. Current native browser WebSocket authentication is blocked by the required Authorization header; a compatible adapter or backend change still needs selection.

Message IDs are server-generated UUIDv7 strings. Messages have conversation-local sequence numbers. Do not order messages by UUID or timestamps. A successful send acknowledges only the client-generated retry ID as a plain text frame; incoming delivered messages are JSON with no event type. There is no REST send endpoint. History omits client retry IDs, so optimistic messages cannot always be reconciled with server metadata deterministically.

History requires a participant record. Active participants retain full history access; left/removed participants can read through their current last-read sequence, inclusive. They cannot send new messages. A missing read sequence means empty history. Nonparticipants receive 403. Delayed read receipts after leaving remain valid and can advance subsequent history access. The active conversation list omits inactive conversations, and participant lookup is active-only; the frontend must not assume those endpoints expose historical membership status.

Planned Markdown chat has nested, consecutive sequence-range sections; paired-null boundaries for empty sections; and section creation chronology recorded by created_at. Ordinary sends do not extend sections. Section-targeted sends must identify their target, and only valid consecutive extension is allowed. Existing adjacent messages can be assigned through boundary operations with automatic hierarchy adjustment. A prior empty section cannot take a new tail message after a later section begins. These controls are planned frontend behavior, not current request fields.

Typing [[parent#child]] opens same-conversation autocomplete. Selecting a result stores a standard Markdown link [display label](section ID) in the message. The target ID disambiguates repeated headers; derived section-reference rows support reverse lookup/search. The user has confirmed standard link ordering. When a header is renamed, the frontend asks whether to update reference labels in the user's older messages. Yes updates persisted message content; no leaves the old labels while links still target the same IDs. Backend rename/edit/bulk-update APIs are not yet designed. Exact author scope, rename batching/failure handling, and parent-path propagation need final contracts; the working interpretation of "their older messages" is messages authored by the acting user, not permission to rewrite other authors' content.

Hashtags are exact-case, exact-spelling conversation-local identities, searchable across nonconsecutive messages. Replies retain their exact reply_to_message_id and remain independent timeline messages; each message can show all reply descendants in a flat collapsible thread, including replies outside its section. Section, hashtag, edit, and thread APIs are not implemented. File-first conversation storage has been scrapped; CRDTs, MinIO, and Elasticsearch have not been adopted.

Latest confirmed constraints: repeated mentions of the same hashtag or target section in one message create one association per distinct message/target pair. Hashtags have no click action; users explicitly search their exact strings. Parent and child sections may share identical boundaries, but overlapping sibling/unrelated section creation is rejected. These decisions are documented behavior for the future feature, not implemented routes.

Physical-schema planning additionally confirmed that historical nonempty sections appear by message sequence ranges rather than their later creation times. Deleting a heading preserves messages and promotes its direct child headings; links to the deleted section remain visible but unavailable. The optional reference-existence boolean is omitted. The backend document's section 9.7 contains proposed SQL/indexes, validated in a rolled-back PostgreSQL transaction, not an applied migration or available frontend API.

The user requires thorough frontend flows, schemas, responses, and handling for every implemented API, with this document maintained as design evolves. This documentation task does not authorize building frontend screens or implementing unfinished backend features.

## 3. Application Areas and Client State

### 3.1 Screens and Panels

| Area | Current capability | Planned addition |
| --- | --- | --- |
| Session restoration | Refresh/login and loading state | None required for Markdown specifically |
| Signup/login | Forms, field errors, session establishment | None |
| Conversation list | Active conversations, participant-derived labels | Section-aware previews when supported |
| New conversation | Exact-email lookup and direct conversation creation | Group management remains blocked on backend |
| Conversation timeline | History, pagination, send status, receipts | Sections, reference navigation, tags, reply threads |
| Conversation details | Other active participants and presence | Section outline and permitted management actions |
| User action menu | Block user | Unblock is unavailable |
| Search | User lookup only | Conversation-scoped section/content/tag/reference searches |
| Section controls | Unavailable | Create, group history, collapse, plus, drag boundary, rename |

Recommended route identities are based on stable conversation IDs, section IDs, and message IDs. Route spelling is a frontend choice; these are not backend routes. Preserve an intended conversation location across login and restore it only after checking server access.

### 3.2 Recommended Stores

- Session: access token, refresh-token storage handle, user ID, device ID, expiration, restoration/refresh state, and session generation.
- Conversations: keyed by conversation ID with current API participant summaries; derived display name is separate from server-provided metadata.
- Messages: keyed by message ID, with per-conversation sequence-ordered indexes. Keep pending sends separately by client_message_id.
- Cursors: loaded sequence ranges, oldest/newest loaded sequence, contiguous synchronization position, read position, and loading/error flags per conversation.
- Connection: disconnected/connecting/connected/reconnecting, retry state, and last successful synchronization.
- Drafts: conversation ID plus optional future section/reply intent. Switching conversations must not leak intent into the next composer.
- Future section tree: keyed by section ID with boundaries, parent, headers, and timestamps. Collapse state is local UI state until a synchronization policy is agreed.
- Future searches: query, conversation ID, filter type, pagination, request generation, and results. Discard stale responses after query or conversation changes.

IndexedDB is a possible client cache/draft store, not an adopted CRDT or offline-sync architecture. Namespace persisted data by account. Recommended logout behavior clears sensitive cached conversations and pending sends, especially on shared devices. Keep device identity separate from account/session data because returning-device login verifies ownership.

## 4. Transport and API Client Rules

### 4.1 HTTP Configuration

- Current default base URL: `http://localhost:8080`; use deployment configuration rather than hardcoding it throughout components.
- Current browser CORS origin: exactly `http://localhost:3000`.
- JSON body requests use `Content-Type: application/json`.
- Protected calls use `Authorization: Bearer <access_token>`.
- Only POST signup, login, and refresh are public through auth middleware.
- Refresh token is the raw `X-Refresh-Token` header value, without a Bearer prefix; send it only to refresh.
- CORS exposes `X-Refresh-Token` and permits Authorization, Content-Type, and X-Refresh-Token headers. OPTIONS returns 204.
- Preserve the exact trailing slash conventions in the endpoint inventory. Do not depend on redirects to repair paths.
- Successful JSON responses have `application/json`; errors are generally plain text from `http.Error`, usually with a trailing newline. Parse by status/content type; trim error text for display.
- Do not call `.json()` on 204 or an empty response. A response's status determines success; do not infer it from an empty body.

The current backend returns refresh tokens in headers, not cookies. A browser client can read them; do not assume HttpOnly-cookie protection exists. Recommended baseline is in-memory tokens, with a deliberate storage adapter if reload persistence is required. IndexedDB/local storage is not equivalent to HttpOnly storage. Native clients can use secure platform storage. Persistent browser-session strategy remains a deployment decision, not a reason to pretend a cookie endpoint exists.

### 4.2 Shared Response Handling

| Result | Client behavior |
| --- | --- |
| 2xx expected schema | Update the relevant store; validate fields needed by the flow |
| 400 | Show request/form error; preserve user input; do not refresh authentication |
| 401 on protected call due to expired/invalid access token | Use one coordinated refresh, retry the original request once, then surface failure |
| 401 on login/refresh | Handle as that endpoint's result; never recurse into another refresh |
| 403 | Show operation/conversation access denial; do not globally sign out or repeatedly refresh |
| 404 | Show endpoint-specific not-found state |
| 409 | Resolve endpoint-specific conflict; avoid blind mutation retries |
| 500/network error | Show recoverable failure; preserve draft/state; retry deliberately according to operation semantics |

Current middleware error bodies include `missing Authorization header`, `invalid Authorization header`, and `invalid token`. Missing header/claims after restoration is usually client session state needing repair, not proof that a successful refresh occurred.

Refresh coordination must be single-flight within a client session; multiple tabs sharing one refresh token also require coordination. The server rotates tokens and treats reuse as suspicious, revoking user sessions. If refresh succeeds, replace access and refresh tokens together before resuming queued calls. Do not replay an older refresh token after losing a response without recognizing the resulting ambiguity. A transient 500/network refresh failure should offer retry; a definitive refresh 401 ends the session.

Guard async responses with a session generation so responses from a logged-out user cannot repopulate another session's state.

## 5. Current Endpoint Inventory

`id` in a path is a conversation UUID. UUID placeholders below are strings, not literal values to submit.

| Method and path | Authentication | Implementation |
| --- | --- | --- |
| POST `/auth/signup/` | Public | Current |
| POST `/auth/login/` | Public | Current |
| POST `/auth/refresh/` | Refresh header | Current |
| POST `/auth/logout/` | Bearer | Current |
| GET `/users/?email=...` | Bearer | Current, exact email only |
| POST `/users/block/` | Bearer | Current |
| GET `/conversations` | Bearer | Current, active membership only |
| POST `/conversations` | Bearer | Current, two participants |
| GET `/conversations/{id}/participants` | Bearer | Current, caller must be active |
| GET `/conversations/{id}/messages` | Bearer | Current, operation-specific history policy |
| POST `/conversations/{id}/messages/read-receipt` | Bearer | Current |
| GET `/ws/` upgrade | Bearer during handshake | Current; browser integration unresolved |
| POST `/conversations/participant` | Bearer | Stub |
| DELETE `/conversations/participant` | Bearer | Stub |
| DELETE `/conversations` | Bearer | Stub |
| DELETE `/conversations/leave` | Bearer | Stub |
| GET `/swagger/` | Outside application auth wrapper | API documentation, not a product action |

There is no current profile/me endpoint, REST send endpoint, message-by-ID endpoint, message edit/delete endpoint, thread endpoint, section endpoint, hashtag endpoint, text-search endpoint, unblock endpoint, or blocked-users list endpoint. Generated Swagger may lag source; handler/service behavior is authoritative for this document.

## 6. Authentication and Account Flows

### 6.1 Signup — Current

User enters name, email, and password. Perform useful client validation, but do not invent backend validation responses; the current handler primarily decodes JSON and the service creates the account.

```http
POST /auth/signup/
Content-Type: application/json
```

```json
{"user_name":"Ruu","email":"ruu@example.com","password":"example-password"}
```

Success: HTTP 201, header `X-Refresh-Token: <token>`, body:

```json
{"access_token":"<jwt>","user_id":"<user-uuid>","device_id":"<device-uuid>"}
```

The backend derives device metadata from User-Agent; the form does not send device_name/device_type. Save session data and the returned device ID, clear password input, navigate to conversations, load the list, then establish a supported socket connection.

| Failure | UI handling |
| --- | --- |
| 400 `invalid request body` | Show form/request failure; retain non-password form fields |
| 409 `email already exists.` | Explain that email is registered; offer login |
| 500 `internal server error` | Keep form; show retry action; if signup outcome is uncertain, allow login rather than blindly creating repeatedly |

### 6.2 Login — Current

New device:

```http
POST /auth/login/
Content-Type: application/json
```

```json
{"email":"ruu@example.com","password":"example-password"}
```

Known device adds `"device_id":"<previous-device-uuid>"`; omit it when no applicable device identity exists. The server verifies device ownership. Do not send another account's stored device ID blindly.

Success: HTTP 200 with the same body fields and refresh header as signup. Replace the local session, load conversations, and connect. Logging in with a known device revokes its previous refresh session.

| Failure | UI handling |
| --- | --- |
| 400 `invalid request body` | Form/request error |
| 401 `invalid credentials` | Explain credential mismatch; do not attempt refresh |
| 401 `unknown device` | Clear the unusable device identity; allow one login attempt without it |
| 404 `Not existing user.` | Explain no account matches; offer signup |
| 500 `internal server error` | Show retry; do not automatically classify every 500 as an unknown device |

Current limitation: the device repository returns a raw no-row error for an unknown device, so that case can surface as 500 instead of the handler's intended 401. A deliberate "log in as a new device" recovery can omit the old ID; do not silently repeat indefinitely. Client-facing wording need not reproduce the backend's punctuation/capitalization.

### 6.3 Restore and Refresh — Current

At boot, load the chosen session storage adapter. If no refresh/access state is available, show login. If an unexpired access token is available, continue; otherwise refresh once. Decoding JWT claims helps obtain user_id/device_id/exp for client state but is not authorization; the backend verifies requests.

```http
POST /auth/refresh/
X-Refresh-Token: <current-refresh-token>
```

No request body is required. Success: HTTP 200, new `X-Refresh-Token`, body:

```json
{"access_token":"<new-jwt>"}
```

Refresh omits user_id/device_id from the response. Preserve them from the established session or decode the new token. Access tokens last 15 minutes; newly created refresh sessions expire after 30 days. Replace the refresh token every successful rotation.

| Failure | UI handling |
| --- | --- |
| 401 `refresh token header not found` | Session cannot refresh; return to login |
| 401 `Invalid refresh token.` | Clear invalid session and show login |
| 401 `Session expired. Please Log in.` | Explain expiration and show login |
| 401 `Suspicious token reuse.` | Clear session; require login; do not retry the same token |
| 500 `internal server error` or network failure | Show recoverable restoration failure; preserve input/drafts; avoid concurrent rotations |

After successful restore, fetch conversations and history independently of socket availability. A successful refresh does not cause a current server-side push event or automatically replace an existing socket's handshake identity.

### 6.4 Logout — Current

```http
POST /auth/logout/
Authorization: Bearer <access-token>
```

No body. Success: HTTP 204. The server revokes the device's refresh sessions; the client closes its socket and clears tokens, user state, pending sends, and account-scoped caches. Keep device identity only if the chosen returning-device policy calls for it. Notify other tabs of the local session change when applicable.

Failures include common 401/missing claims and 500 `action failed`. If a user chooses local sign-out despite a failed request, do not claim server revocation succeeded. Current logout does not itself close all server sockets or invalidate already-issued JWTs immediately; the frontend must close its own connection and stop retries.

## 7. User Search and Blocking

### 7.1 Find a User — Current

In "new conversation", enter an email and submit an encoded query:

```http
GET /users/?email=ruu%40example.com
Authorization: Bearer <access-token>
```

Success HTTP 200:

```json
{"id":"<user-uuid>","user_name":"Ruu","email":"ruu@example.com"}
```

This is one exact-match object, not a paginated list or prefix autocomplete API. The `username` query is mentioned in annotations, but only email lookup is implemented. Preserve email spelling unless a separate normalization decision is made.

Responses: 400 `either email or username must be provided` when email is absent, 404 `user not found`, common 401, or 500 `internal server error`. Show a no-results state for 404. Disable starting a conversation with the current user; the backend also rejects self-conversation. Ignore stale results if the query changed.

### 7.2 Block a User — Current

Offer a user action with the selected target identity. On confirmation:

```http
POST /users/block/
Authorization: Bearer <access-token>
Content-Type: application/json
```

```json
{"blocked_user_id":"<target-user-uuid>"}
```

Success: HTTP 204. Failures: 400 `invalid request body`, common 401/missing claims, 500 `user not found` for an absent target, or 500 `internal server error` (including duplicate insertion). Display success only for the actual 204 response; on failure keep the action retryable without assuming the user was blocked.

The current operation stores a blocked-user relationship; message send/recipient selection does not consult it. Do not promise that it immediately suppresses messages. There is no unblock/list-blocked endpoint to restore this UI state after reload. The handler attempts a 204 even after writing an error, but the already-written HTTP error status remains the observable failure; follow that status.

## 8. Conversation Flows

### 8.1 List Conversations — Current

```http
GET /conversations
Authorization: Bearer <access-token>
```

Success HTTP 200:

```json
{"conversations":[{"conversation_id":"<conversation-uuid>","participants":[{"participant_id":"<user-uuid>","participant_name":"Ruu"}]}]}
```

Load after authentication and refresh after successful creation or manual reload. Empty state is `{"conversations":[]}`. For a direct chat, derive the display name from participants other than the current user. The list includes the caller's participant summary and may include inactive others; do not treat it as the active-presence list.

The caller's membership must be active for a conversation to appear. Left/removed history may remain available via a known conversation ID despite disappearing from this list. There is no inactive-conversation discovery endpoint yet.

The response lacks type, title, avatar, last message, unread count, own role/status, and last-read sequence. Do not invent those server fields. The service groups using a map, so a stable recent-first list order is not guaranteed even though the repository query orders rows. A client may maintain a clearly local last-activity order from known messages.

Failures: common 401/missing claims, 500 `failed to fetch conversations`, or generic 500. Keep successful cached results distinguishable from a fresh response and offer retry.

### 8.2 Start a Conversation — Current

First perform user lookup, then:

```http
POST /conversations
Authorization: Bearer <access-token>
Content-Type: application/json
```

```json
{"other_user_id":"<target-user-uuid>","type":"direct"}
```

Type is optional and defaults to direct. The authenticated user is supplied by the backend; do not send RequestingUserID.

Success HTTP 201: `{"conversation_id":"<conversation-uuid>"}`. Refresh the conversation list and open that ID.

| Failure | UI handling |
| --- | --- |
| 400 `invalid request body` | Preserve selected target and show request error |
| 400 `cannot start conversation with yourself` | Explain and return to user selection |
| 400 `invalid conversation type` | Client configuration error; do not retry unchanged |
| 409 `direct conversation already exists between the two users` | Reload conversations, find the matching participant pair, and open it if unambiguous |
| 401/missing claims | Shared session handling |
| 500 `internal server error` | Show retry; target absence and some database failures are not differentiated |

The duplicate response does not return the existing conversation ID. The list also omits type, so matching a pair can be ambiguous when group-typed conversations exist. Do not select an arbitrary match; show candidates or retain a conflict state if the frontend cannot resolve it.

Although `type: "group"` is accepted, creation still adds only the caller and one other user. Do not present a fully functioning multi-user creation wizard against this API.

### 8.3 Participants and Presence — Current

```http
GET /conversations/<conversation-uuid>/participants
Authorization: Bearer <access-token>
```

Success HTTP 200:

```json
{"participants":[{"participant_id":"<other-user-uuid>","participant_name":"Ruu","is_online":true}]}
```

Only active participants are returned and the caller is excluded. Normalize `participants: null` to an empty client list. Fetch when opening details or refreshing presence. There is no proactive presence event; periodic polling while a view is visible is a frontend recommendation, with interval still to be chosen.

Presence is a heartbeat-refreshed Redis key with a 120-second TTL. Offline status can lag actual disconnect. Do not label it as exact last-seen time or immediate disconnect detection.

Failures: 400 `id path parameter is required` / `invalid id in path`, common 401, 403 `the user is not an active participant of the conversation`, or 500. On 403, hide active-management/presence controls, but still try history if appropriate: inactive participants can have legitimate bounded history access.

### 8.4 Group Management — Stubs

POST/DELETE `/conversations/participant`, DELETE `/conversations`, and DELETE `/conversations/leave` are registered but empty. An authenticated call can return an empty 200 without changing anything. No functional request or success schema exists. Hide/disable these actions until their backend behavior and permissions are designed; never interpret that empty 200 as successful membership deletion.

## 9. Timeline, History, and Read Receipts

### 9.1 Fetch History — Current

```http
GET /conversations/<conversation-uuid>/messages?limit=100
Authorization: Bearer <access-token>
```

Query fields: positive integer `limit` (default 100), optional exclusive `before_seq`, optional exclusive `after_seq`. Do not combine the two cursors. There is no current server maximum limit; choose bounded page sizes in the client. Sequence values are int64 in Go; the eventual frontend numeric representation must account for JavaScript safe-integer limits if histories can reach them.

Success HTTP 200:

```json
{
  "conversation_id":"<conversation-uuid>",
  "limit":"00000000-0000-0000-0000-000000000000",
  "messages":[{
    "message_id":"<server-uuidv7>",
    "conversation_id":"<conversation-uuid>",
    "sequence_no":42,
    "sender_user_id":"<user-uuid>",
    "sender_name":"Ruu",
    "payload":"hello",
    "timestamp":"2026-10-23T10:00:00Z"
  }]
}
```

The `limit` response field is currently mistakenly typed as UUID and left at its zero value. Ignore it. The message array can be null; normalize null to empty. History does not expose reply targets, edited timestamps, client_message_id, section metadata, or receipt status. UUIDv7 IDs are identifiers; order by sequence_no.

**Permission policy:** active participants receive normal history; left/removed participants receive only sequences at or below their current last_read_mssg_seq. The server applies that cap before sorting and limiting. No recorded read sequence means no history. Nonparticipants receive 403. A response does not explicitly identify the caller's status or cutoff; a short page alone must not be interpreted as proof of removal.

| Failure | Client behavior |
| --- | --- |
| 400 `id path parameter is required` / `invalid conversation_id` | Show invalid route; do not send repeated requests |
| 400 `invalid limit` / `invalid before_seq` / `invalid after_seq` | Repair request parameters |
| 400 `before_seq and after_seq cannot both be present` | Client pagination error |
| 401 including `missing auth claims` | Shared session handling |
| 403 `user is not a participant of the conversation` | Conversation unavailable; keep session valid and stop requesting this route |
| 500 `internal server error` | Preserve loaded content with an error/retry state |

### 9.2 Open a Conversation

1. Set active conversation and restore its draft without carrying another conversation's section/reply target.
2. Fetch latest history. Fetch participants separately; their access outcomes can differ.
3. Render ascending sequences, derive ownership from sender_user_id, and use server timestamps only for display.
4. Merge live incoming messages into the same store. Deduplicate by message_id and conversation sequence.
5. Send a receipt for the highest actually viewed/read message, not merely the newest downloaded message.
6. Show independent loading/failure states for history, participants, and the socket.

For a known inactive conversation, preserve bounded read access even when presence returns 403 or the active list omits it. The current API does not provide sufficient membership metadata to implement a perfectly authoritative read-only badge; treat known status as state, not an inference from one empty response. Sending remains server-enforced.

### 9.3 Load Older Messages

Request `?before_seq=<oldest-loaded>&limit=50`. Prepend/merge while preserving scroll position. Stop for that history boundary when an empty page arrives; permit explicit refresh if the server's accessible history changes later. Avoid concurrent duplicate requests for the same boundary.

### 9.4 Catch Up After Reconnect or Resume

Request `?after_seq=<last-contiguous-synchronized-sequence>&limit=100`, merge pages in ascending order, and continue until no more accessible messages remain. Live frames can arrive between requests; deduplicate them.

Do not advance the catch-up cursor merely to the largest live sequence if lower messages are still missing. Example: receiving 105 while only synchronized through 100 still requires fetching after 100. Track missing ranges separately from the largest observed sequence. Initial latest-page loading is a bounded window, not proof that all earlier history was downloaded.

Message-only catch-up will not recover future historical label edits, section renames, or boundary changes. Their eventual APIs/events need a separate resynchronization contract.

### 9.5 Submit Read Receipt — Current REST

```http
POST /conversations/<conversation-uuid>/messages/read-receipt
Authorization: Bearer <access-token>
Content-Type: application/json
```

```json
{"message_id":"<read-message-uuidv7>","sequence_no":42}
```

Success HTTP 204. Batch/debounce to the highest visibly read message per conversation; keep the message ID and sequence from the same server message. Lower positions do not move the stored read position backward. A successful receipt is not an event that other participants currently receive.

Errors: common 401/missing claims; 400 invalid conversation/path/body; 400 `message_id is required`; 400 `sequence_no must be positive`; 500 `internal server error`. Retain an unsent highest-read position for a controlled retry, without marking unseen content read. Delayed receipts remain valid after leaving/removal; do not discard them solely because membership changed. Such receipts can increase future accessible history.

## 10. WebSocket Contract and Sending

### 10.1 Connect — Current Backend, Browser Adapter Needed

The server upgrades `/ws/` with `Authorization: Bearer <access-token>`. It derives user and device IDs from that token. Query-token and cookie authentication are not implemented.

A native browser WebSocket cannot set that custom header. A native client capable of headers can connect; a browser needs a deliberately chosen proxy/backend authentication adaptation. Do not ship a browser implementation that silently assumes `?token=...` works. Show history/read functionality independently while this integration is unavailable.

Successful upgrade is 101. Auth failures are HTTP 401; upgrade failures are handshake errors. Registration failure after upgrade closes the socket with code 1011 and reason `backend registration failed`. The server supports one current connection per device: reconnecting the same device replaces the earlier connection. Multiple tabs need an ownership strategy to avoid reconnect loops; a shared leader connection is a recommendation, not an existing feature.

The server sends protocol pings about every 54 seconds, with a 60-second pong deadline and a 64 KiB incoming frame limit. Browser WebSocket implementations handle protocol pongs; no application JSON heartbeat is required. Reconnect with backoff/jitter, stop on local logout, and refresh credentials when the handshake requires it. An established socket currently does not revalidate token expiration on each frame.

### 10.2 Send a Message — Current

```json
{"type":"message","conversation_id":"<conversation-uuid>","payload":"hello","client_message_id":"<client-generated-uuid>"}
```

The client does not submit sender/device IDs, server message ID, sequence, or timestamp. Generate client_message_id before optimistic rendering and preserve it when retrying the same send. The server requires a nonempty payload and valid nonzero conversation/client IDs. Sending requires current active participation.

Recommended pending states are draft, queued, sending, acknowledged, uncertain, and failed. Offline composition can stay local; there is no current server-side REST fallback. Do not present a locally queued message as persisted. Validate the encoded frame size, since metadata and Markdown links also consume the 64 KiB limit.

On success, the server sends only a text UUID matching client_message_id. It does not send the JSON SendMessageAck struct, server message ID, sequence, or timestamp. Mark the pending item acknowledged, not read/delivered to all recipients.

The message is routed to other active participant devices and the sender's other devices, excluding the originating device. The originating optimistic item therefore lacks final metadata until another read supplies it. Current history omits client_message_id, so content/time matching cannot safely establish identity when duplicate-looking messages exist. Keep that reconciliation limitation explicit; do not silently merge two messages just because their text matches. A future full acknowledgment or history retry ID is a backend dependency, not a current contract.

A timeout is uncertain: the message may have persisted before a publication error. Database deduplication prevents repeated insertion for the same sender/client ID, but the current duplicate error checks construct new errors and do not reliably return the intended duplicate frame. Do not promise that retrying always produces an acknowledgment or republishes a previously stored message. Offer retry/catch-up with clear pending state rather than infinite automatic resends.

### 10.3 Receive a Message — Current

```json
{"message_id":"<uuidv7>","conversation_id":"<conversation-uuid>","sequence_no":42,"sender_user_id":"<user-uuid>","sender_name":"Ruu","payload":"hello","timestamp":"2026-10-23T10:00:00Z"}
```

There is no `type` field. Parse JSON frames as delivered messages only when the required fields match; process plain text separately. Update the message store and local preview/unread estimates. If the user is viewing and has read it, advance the receipt. Do not mark an offscreen or collapsed message read merely because its frame arrived.

### 10.4 WebSocket Read Receipt — Current

```json
{"type":"read_receipt","conversation_id":"<conversation-uuid>","read_message_id":"<uuidv7>","read_message_seq_no":42}
```

There is no acknowledgment or receipt broadcast. Errors are logged on the server. Use REST when a confirmed receipt submission response is needed or the socket is unavailable. Both paths preserve delayed receipts and monotonic read positions.

### 10.5 Errors and Unimplemented Types

Current validation error frames are plain text: `missing payload`, `missing conversation_id`, and `missing client_message_id`. Invalid JSON, missing type, database errors, and inactive-sender rejection can be logged without a client error frame. The intended `message already exists` branch is unreliable as noted above. An unknown future type may be ignored rather than rejected; sending it is not evidence of support.

Typing, edit_message, delete_message, section fields/events, and reply-to metadata are not implemented incoming behavior. Existing structs/comments are not sufficient API support. A future event envelope must be introduced explicitly; do not require current delivered messages to contain one.

## 11. Markdown Message Rendering and Section References

### 11.1 Render Every Message — Confirmed Future Frontend Behavior

Render each message payload as Markdown. Its own headings are message-body content, distinct from conversation-level section objects. Render ordinary links and selected section links appropriately; recognize internal section targets from the eventual agreed target convention. Do not interpolate arbitrary raw HTML as trusted application markup. A specific parser/editor library has not been selected.

Raw payload storage currently works, but section-aware parsing, reference validation, and derived association writes are not implemented. The normal message API can carry Markdown text now; that does not make autocomplete/navigation/search available.

### 11.2 Compose a Section Reference — Confirmed Future Behavior

1. While typing `[[...]]`, request matching section headers/path labels in the current conversation using the future section lookup API.
2. Show header, parent path, and enough context to distinguish duplicate names; selection binds an ID, not just a string.
3. Serialize using **standard Markdown ordering**, for example `[Parent section#Child section](019a0b00-0000-7000-8000-000000000001)`, substituting the selected section's actual ID.
4. The wire message payload and persisted content contain the display label and target UUID together. The backend derives section_references from actual link nodes.
5. The rendered message displays the label; clicking uses the embedded ID without fetching associations to pair occurrences.

The actual target string is a UUID in the proposed content model; whether to add a distinguishing URI prefix remains open. Avoid confusing ordinary web links with internal section links. Escape labels correctly rather than splitting message text with simple delimiter replacement. Do not transform apparent references inside code spans/blocks without a defined parsing rule.

Repeated display labels can link to different IDs, and multiple mentions of the same ID can coexist. The backend will store one association per distinct message/target-section pair, not one per link occurrence. Rendering still preserves and resolves each individual inline occurrence.

### 11.3 Follow a Reference — Confirmed Future Flow, API Pending

Read its target ID, resolve that section within the current conversation, expand collapsed ancestors, load the relevant message range, scroll to the heading, and optionally highlight it. Preserve back-navigation to the originating message. If resolution fails or access is denied, keep the original message visible and show an unavailable-target state. A duplicate or stale label must never redirect by title to another section.

Required backend capabilities: section-by-ID resolution with hierarchy/bounds, lookup/autocomplete, and section-content retrieval or bounded timeline retrieval. Exact methods/paths/schemas/statuses are not defined yet. Existing before/after queries alone do not return an empty section or its ancestor chain.

## 12. Section Flows — Confirmed Behavior, Backend APIs Pending

### 12.1 Create Before Discussing a Topic

User chooses a heading and optional parent. The future create operation returns the section identity, parent, timestamps, and nullable boundaries. Initially both start_sequence_no and end_sequence_no are null. All new tables have created_at and updated_at; created_at retains section creation chronology. Exact endpoint and response are pending.

Keep the heading visible even while empty. Creation does not force an ordinary composer to target it. A failed creation retains the input and does not create a fake server ID. For concurrent creation, refresh authoritative structure; tie handling and nested last-section eligibility remain open.

### 12.2 Group Historical Messages

Select a consecutive message range and choose a header/parent. Show the selected first/last sequences before confirmation. Submit sequence boundaries and intended parent to a future backend operation; never renumber messages or select by timestamps/UUID ordering. A child may cover exactly its parent's range. Overlapping sibling/unrelated creation is rejected, not resolved by automatically changing existing sections. On rejection retain the selection, explain the conflict, and let the user revise it or choose valid nesting. Server validation remains authoritative; exact request and error response are pending. Explicit boundary transfers retain their separately agreed automatic propagation behavior.

Place nonempty headings using their message ranges, even when created later around historical content. For D = 1–9 created after A = 10–20 and C = 21–30, render D before A; C remains the tail section for a possible explicit message 31. `created_at` still records creation chronology and is relevant for empty headings; do not blindly sort all nonempty sections by it.

### 12.3 Send Through the Section Plus Control

Show the action only when the section is eligible for a consecutive tail append. Selecting it places explicit section intent in the composer; show the target heading and allow cancellation. Ordinary sends remain ordinary.

The future send contract needs a section identity, recommended as optional section_id; this field is not currently handled. On acceptance the message and all affected boundaries update together. A concurrent ordinary send can break adjacency: retain the draft on rejection and let the user choose ordinary send or revise the target. Do not automatically include intervening unrelated messages.

For A = 10–20, B = empty, C = 21–30 created in that order, message 31 cannot target B. An empty section is not universally eligible simply because its boundaries are null.

### 12.4 Drag a Boundary or Adjacent Message

Preview only consecutive transfers. If A ends at 20, existing 21 may transfer from adjacent B; 23 cannot skip 21–22. Submit the existing message/range and source/target section identities using a future contract. Message IDs and sequences remain unchanged.

Removal propagates from immediate source section outward; insertion propagates from outermost affected destination inward. Reconcile the whole affected hierarchy from the response/event. An emptied section remains visible with paired null boundaries. Preserve other participants' updates on conflict; exact revision/concurrency contracts remain open.

### 12.5 Collapse and Navigate

Maintain collapse state keyed by section ID, independent of duplicate headers or subsequent renames. Nested sections retain their own toggles when ancestors collapse. Reference navigation/search can temporarily expand ancestors; the exact restoration UX is a frontend choice. Collapsed visibility is not permission to omit authorized history from search, nor evidence of messages being read. Cross-device collapse preference storage is not yet required.

### 12.6 Delete a Heading — Confirmed Behavior, API Pending

Present deletion as removing a section heading, not deleting the conversation's messages. On accepted deletion, remove the heading, promote its direct child sections to the deleted heading's parent/root, and retain all message IDs, content, and sequence positions. Preserve surviving child-section IDs and links to those children.

Existing message links targeting the deleted section are not rewritten or redirected by label. Render their original display text as an unavailable target; clicking shows the unavailable state rather than navigating to a duplicate-name section. Do not remove the original link text from messages.

No section deletion endpoint exists yet. Confirmation UI, authorization, exact response/statuses, and events still require API design. On failure retain the current tree and report the error; reconcile only a confirmed result. Backend association-row cascading is an implementation detail, not permission to hide or rewrite message content.

## 13. Rename and Updating Older Reference Labels

### 13.1 Confirmed User Decision

Use standard Markdown `[display label](target ID)` ordering. When a section header is updated, the frontend asks the user whether they want the header updated in their older referencing messages. Yes means **persistently update message content**; no means preserve old labels. This is not just replacing displayed text in memory.

Working interpretation: "their older messages" means messages authored by the acting user in that conversation. Do not automatically rewrite other authors' messages. A broader author scope requires an explicit product/permission decision.

### 13.2 Required User Flow

1. Open the section's rename action and edit its header; retain the stable section ID.
2. Present an explicit choice such as "Update reference labels in your older messages too?" with update/keep choices. Recommended timing is before final submit, so intent can accompany the rename; exact UI timing is not a settled backend contract.
3. If declined, rename the section while leaving existing message payloads unchanged. Links remain navigable by their unchanged target IDs.
4. If accepted, rename and replace the relevant display labels in the user's older message content. Match links by target ID, not by old label text, because headers repeat. Preserve other message text, code blocks, unrelated links, message IDs, send sequences, and target IDs.
5. Reflect only confirmed results in the section outline, open timeline, collapsed-thread views, reference previews, and search caches. Other devices need the eventual metadata/message-update event or refresh path.
6. If the operation fails, retain the new-name input and chosen option. Do not report all older messages updated when the server has not confirmed it.

A section rename does not automatically rewrite every reference in the conversation. Search by current header can still resolve the section and use association rows to find messages retaining an old display label.

### 13.3 API Dependency and Open Contract

There is no section rename, message-edit, or bulk-reference-update endpoint yet. Do not implement this as a loop over currently loaded messages: older references can be on unloaded pages. The backend operation must cover the intended persisted scope.

Required semantic input: conversation ID, section ID, new header, and the user's update-existing-labels choice. Required result information: authoritative renamed section and an unambiguous outcome for affected messages. These are capability requirements, not approved JSON field names or a published API.

Still open: one atomic rename/update operation versus a separately retried bulk edit; partial-result reporting if batching is needed; treatment of labels deliberately customized by authors; parent-header path changes on descendant links; stale concurrent message edits; and exact timestamps/events. Finalize these before a frontend implementer invents behavior. The opt-in choice and persisted-content updates are confirmed.

## 14. Search, Hashtags, and Replies — Future Flows

### 14.1 Section and Message Search

Search stays inside the selected conversation. Provide distinct intents: find section headers, search message content within a selected section, and find messages referencing a selected section. Resolve ambiguous headers to section IDs. Whether section-content search includes descendants and the exact matching/ranking rules remain open.

Reference search uses sections matching the queried name plus their referencing messages; old display labels may differ after a declined rename update. Historical-label text search is a different query. Results should identify target section and source message and support navigation/ancestor expansion. No implemented route or response exists for these searches.

### 14.2 Hashtags

During composition/rendering recognize hashtags according to the eventual parser rules. Preserve spelling/case; do not lowercase, stem, or merge #Go and #go. Same exact literal in one conversation reuses one hashtag identity; another conversation has its own identity.

**Clicking a hashtag does nothing.** Do not attach section-like navigation or click-to-search behavior. To find tagged messages, the user explicitly searches the exact hashtag string within the current conversation. Messages need not be consecutive. Show normal paginated message results and navigate from results by message ID/sequence. Repeated occurrences of one hashtag in one message produce one association and one matching message, not duplicate search results. Backend tag lookup/search/mutation APIs, token syntax, escaping/code behavior, and response fields are pending. Do not send unsupported hashtag arrays and assume association rows are created today.

### 14.3 Reply and Flat Descendant Thread

Choose "reply" on a message, show its preview in the composer, and preserve its exact ID as the future reply target. A reply to a reply keeps that immediate target, not the oldest ancestor. The eventual accepted message remains an independent timeline item with its own sequence.

Expanding a message's thread shows all descendants in one flat collapsible view, regardless of their sections. Example: B replies to A, C to B, D to A; A's thread shows B/C/D, B's thread shows C. Use the same message store for timeline and thread appearances to avoid divergent content after edits.

Section membership is separate from reply intent. Ordinary reply sends do not extend a section; a section-targeted reply must meet the same explicit-intent and adjacency rules as any other send. The current socket read pump does not pass reply_to_message_id through, and current history responses omit it despite a database field existing. Reply sending/retrieval APIs and flat ordering/pagination remain pending.

## 15. Future API and Event Tracking

For every new backend capability, add the exact method/path, auth requirement, JSON request, success response, error statuses/bodies, event shape, frontend transitions, and reconnect behavior here before marking it current.

| Capability needed | Status | Frontend dependency |
| --- | --- | --- |
| Section create/list/resolve/tree | API pending | Outline, empty headings, hierarchy, link navigation |
| Historical grouping and boundary transfer | API pending | Selection, drag preview, affected-range reconciliation |
| Explicit section-targeted send | Field/protocol pending | Plus composer and adjacency error handling |
| Header/path autocomplete | API pending | [[...]] selection and duplicate disambiguation |
| Rename with optional older-label update | API/transaction contract pending | Prompt, persistent updates, complete historical scope |
| Section-content and reference search | API pending | Scoped search panels and navigation |
| Hashtag lookup/search/association handling | API/parser pending | Exact tag interaction |
| Reply send and descendant retrieval | API pending | Quote composer and flat thread |
| Historical edit/section resync | API/event contract pending | Refresh after reconnect or another user's edits |
| Full send acknowledgment/retry-ID history | Improvement proposed, not implemented | Deterministic optimistic reconciliation |
| Browser-compatible socket auth | Integration decision pending | Native browser real-time messaging |

Backend candidate event names include message.created, section.created, section.updated, section.content_updated, and section.deleted; they are not implemented wire formats. Rename propagation also requires message-update visibility, whose exact event remains undefined. Existing live-message catch-up cannot discover historical edits just by asking for larger sequences. A structure/message refresh or change cursor needs an explicit design; do not assume an event log already exists.

## 16. Acceptance Scenarios for the Future Frontend

Current-contract scenarios to validate during frontend implementation:

- Signup/login save tokens from the correct header/body, distinguish form errors, and restore the intended route.
- Simultaneous expired-token requests perform one refresh; refresh failure does not recurse; logout prevents late responses restoring state.
- Exact-email lookup handles empty/not-found results and conversation creation handles duplicate conflicts without inventing an existing ID.
- Stub group actions remain disabled despite empty 200 responses.
- History handles null arrays, ignores the UUID limit field, honors exclusive cursors, and deduplicates concurrent live/catch-up results.
- Active, inactive-with-cutoff, inactive-with-null-cutoff, and nonparticipant history states are handled without confusing 403 with logout.
- Participant-list 403 does not erase legitimate historical access; delayed receipts remain submit-able.
- Socket connection respects current handshake limitations; plain acknowledgments and untyped JSON messages are parsed separately.
- Unknown send outcomes remain uncertain, duplicate-looking messages are not merged by text, and retries keep client IDs.

Confirmed future Markdown scenarios, executable only once their contracts exist:

- Empty sections retain identity and creation chronology; non-last empty sections reject new tail insertion.
- Ordinary sends do not extend sections; plus-targeted sends do; races preserve drafts and show a meaningful rejection.
- Consecutive transfers adjust source/destination ancestors and retain emptied sections.
- Equal-boundary parent/child grouping is allowed, while overlapping sibling/unrelated creation is rejected without silently adjusting existing sections.
- Newly grouped historical sections appear at their sequence ranges; deleting a heading promotes its children without deleting messages, and old links to that heading show an unavailable state.
- Duplicate labels navigate by embedded IDs; references do not depend on positional joins.
- Rename with no preserves older payloads; rename with yes updates the intended user's persisted reference labels beyond loaded pages.
- An unrelated section with the same old header is not rewritten; message sequences and reference target IDs remain unchanged.
- Exact-case tag searches remain conversation-scoped.
- Hashtag clicks cause no action; repeated hashtags and repeated links to one section do not duplicate message-level search results or association rows.
- Replies appear in both timeline and flat descendant views with exact immediate targets and independent section placement.
- Historical edits refresh all cached appearances and recover after a missed event once a resync protocol is defined.

## 17. Change History

### 2026-10-23 — Historical Placement and Heading Deletion Confirmed

- Confirmed range-based placement for historical nonempty sections, keeping later live sections eligible independently of section creation timestamps.
- Added the heading-delete flow: preserve messages, promote direct children, retain child link identities, and display old links to the deleted heading as unavailable.
- Linked the proposed physical database design without inventing implemented section APIs. No frontend code changed.

### 2026-10-23 — Hashtag Search-Only Interaction and Grouping Constraints

- Confirmed hashtags have no click action; explicit exact-string search retrieves matching conversation messages once each.
- Confirmed per-message/per-target association uniqueness while preserving repeated inline occurrences in content.
- Confirmed equal parent/child ranges and rejection of overlapping sibling/unrelated section creation. Updated grouping flow, context summary, and acceptance scenarios.
- No API, schema, or frontend code was implemented.

### 2026-10-23 — Initial Comprehensive Frontend Design

- Created the maintained frontend companion covering all implemented auth, users, conversations, history, receipt, and WebSocket APIs, plus registered stubs and browser integration constraints.
- Recorded request/response schemas, actual error behavior, state transitions, pagination, restoration, refresh coordination, retry limitations, presence, and existing permission differences.
- Recorded confirmed Markdown flows and separated pending API/schema decisions from callable interfaces.
- Confirmed standard Markdown inline-link ordering and the rename prompt controlling persistent updates to older reference labels.
- Included a reusable context summary, capability checklist, and acceptance scenarios. No frontend or backend implementation was performed.
