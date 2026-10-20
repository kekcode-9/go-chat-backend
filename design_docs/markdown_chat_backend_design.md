# Markdown Chat Backend Design

Last updated: 2026-10-20

Status: design discussion in progress. Markdown chat functionality has not been implemented.

## 1. Purpose and Maintenance

This is the continuing design record for conversation-scoped Markdown chat in this repository. It records product requirements, database and backend proposals, reasoning, unresolved decisions, and the context needed to resume the discussion with another engineer or LLM.

Update this document whenever a decision changes or a new decision is added. Keep the context summary, detailed design, decision status, and change history consistent. Record superseded decisions and their replacements rather than silently losing the reasoning.

Distinguish the following statuses throughout the document:

- **Required:** explicitly requested behavior or constraints from the user.
- **Implemented:** behavior verified in the existing backend.
- **User-proposed:** an approach the user has suggested; not necessarily a finalized implementation decision.
- **Recommended:** an approach suggested by the assistant, awaiting agreement where applicable.
- **Open:** behavior or implementation details that have not been decided.

Do not promote a recommendation to an accepted decision just because it appears in this document. Do not implement Markdown chat changes without a request to do so.

## 2. Context Summary for Resuming Work

The project is a Go chat backend using PostgreSQL 17, Redis Pub/Sub, JWT authentication, and Gorilla WebSockets. Messages are persisted before being routed through Redis to connected recipient devices, including the sender's other devices. Each message has a globally unique ID and a conversation-local `sequence_no`. Message history uses sequence cursors for pagination and reconnect catch-up.

The user wants conversations to support a Markdown-like section hierarchy, rather than merely Markdown-formatted message bodies. Consecutive messages can form named sections; sections can nest, collapse, be searched, and be referenced from messages using `[[section_header]]`. Headers may repeat, but section IDs must be unique. Users should be able to create a section before discussing a topic or group historical consecutive messages. The requested UI includes a plus action for adding text to a group and dragging a boundary to include existing messages. Not all messages need to belong to a section.

Messages can also mention any number of hashtags. Hashtags are case- and spelling-sensitive, scoped to one conversation, and identify messages for search rather than contiguous groups. The same exact hashtag name in one conversation refers to one hashtag identity; the same name in another conversation is independent. Replies remain independent messages in the chronological conversation and also appear in a collapsible thread attached to the message being replied to. They do not inherit the target message's section membership.

The user is concerned about the growth rate of the messages table and preserving one message row per unique conversation sequence number. Association rows must live in separate tables: they must never duplicate messages or consume message sequence numbers. The user's proposed structure is a sections table with conversation ID and start/end message sequence numbers, a separate `section_references` table linking messages to sections, and analogous tables for hashtags. They also proposed a `has_section_references` boolean and a reference to section headers. The assistant recommends omitting the boolean initially and referencing a section only by its stable ID; those adjustments have not yet been explicitly accepted.

The user has now answered the three initial behavior questions:

1. Section extension requires consecutive message sequence numbers. If A ends at 20, message 21 can be included next; 23 cannot be included while 21–22 remain outside A. The plus action is unavailable when a section has ended and another has begun immediately after it. A newly sent message cannot be inserted visually into an earlier section across intervening messages. Moving the first message of adjacent B into A changes section boundaries, not the message's identity or chronological position. The frontend controls come later; the backend must support and validate their effects now in the design.
2. Typing `[[header]]` shows matching sections, the user selects one, and the reference stores the chosen section ID. Headers may repeat.
3. Replies have their own message rows and conversation sequence numbers and appear in the normal timeline as well as in a collapsible reply thread. Preserve the exact `reply_to_message_id`, including when replying to a reply. A message's thread includes all descendants, flattened into one view, regardless of their sections. Reply targets do not confer section membership.

On 2026-10-20 the user confirmed that empty sections are allowed, including after losing their final message. Nested boundaries adjust automatically: removal updates the immediate owning section first and proceeds outward through ancestors; insertion updates the outermost affected section first and proceeds inward toward the target. Ordinary sends never automatically extend a section. Sending through the future plus control must explicitly identify the target section in the incoming message; alternatively an ordinary sent message can later be assigned by a valid consecutive boundary operation. An optional `section_id` is the assistant's recommended wire representation; a separate boolean is not required to distinguish targeted and ordinary sends.

These decisions support sequence-range section membership and remove the need for a separate document-order model for the requested behavior. Empty-section storage/placement, exact nested-boundary algorithms, and protocol details still need specification before implementation. The assistant recommended PostgreSQL, separate relational association tables, and a distinction between section membership and section mentions. No graph database or JSONB message-content change has been agreed.

A preceding, separate task implemented UUIDv7 for new server-generated message IDs using the existing `github.com/google/uuid` dependency. Go and PostgreSQL continue to use `uuid.UUID` and `UUID`. The initial migration and standalone schema no longer generate UUIDv4 message IDs by default; seed messages have explicit UUIDv7 IDs. Existing-ID conversion was unnecessary because the application is not yet in production and can start with a fresh database.

The user requires this detailed design document, including this reusable context summary, to be kept current as the discussion evolves. The latest update records permitted empty sections, automatic directional boundary propagation, explicit section targeting for sends, and exact reply targets with flat recursive-descendant threads. Continue the design conversation without treating remaining unanswered questions as settled or implementing the feature prematurely.

Read-receipt policy clarification: a delayed receipt for a message received while the user was an active participant remains valid after the user leaves or is removed. Recording that read is expected behavior. Do not classify the absence of an active-status check in the receipt path as a defect or add such a restriction as part of this design.

Message access policy is now implemented: history requires a participant record, returning HTTP 403 for nonparticipants. Active participants retain the existing history behavior. Left/removed participants can fetch only through their current `last_read_mssg_seq`, inclusive, with the bound applied before pagination and limits. A missing last-read sequence means empty history. Sending requires active membership checked before sequence allocation and insertion. Accepted delayed receipts can advance an inactive participant's history cutoff. Services own these decisions, repositories own SQL, handlers own transport responses, and domain errors remain in the message module's errors file. No Markdown feature or schema change was needed for this access-policy update.

## 3. Existing Backend Baseline

### 3.1 Organization and Message Flow

- `internal/message` owns message persistence, history, read receipts, publishing, and subscription.
- `internal/conversations` owns conversation creation, listing, and participant lookup.
- `internal/websocket` owns connections, read/write pumps, and device routing.
- `internal/platform/models` contains shared transport models.
- `cmd/server` wires services, PostgreSQL, Redis, HTTP routes, and workers.

Incoming WebSocket messages are validated and passed to the message service. Within a PostgreSQL transaction, the service atomically allocates a sequence number by updating the conversation counter and inserts one message. After commit, it finds participant devices, groups online devices by backend, and publishes one outgoing payload per destination backend. Subscribers route payloads to local WebSocket connections.

### 3.2 Existing Message Identity and Ordering

Implemented message fields relevant to this design include:

| Field | Meaning |
| --- | --- |
| `id` | Server-generated UUIDv7 primary key |
| `conversation_id` | Conversation owning the message |
| `sequence_no` | Authoritative conversation-local send order |
| `sender_id` | Author |
| `content` | Text body |
| `client_message_id` | Client-generated retry/deduplication identifier |
| `reply_to_message_id` | Nullable reference to another message; complete thread behavior is not implemented |
| `created_at`, `updated_at`, `deleted_at` | Creation, edit, and soft-deletion fields |

The active initial migration has `UNIQUE (conversation_id, sequence_no)` and `UNIQUE (sender_id, client_message_id)`. Neither section-reference rows nor hashtag associations may alter these invariants.

The standalone `schema.sql` has a pre-existing difference: its client-message uniqueness constraint does not match the repository's current conflict target. This was observed during UUIDv7 validation and left unchanged as unrelated. Use the migration as the current application baseline when reasoning about this constraint.

### 3.3 Existing Interfaces and Limitations

- Message history supports latest messages and `before_seq` / `after_seq` cursors, returning messages in ascending sequence order.
- Read receipts advance a participant's stored read position.
- Current incoming WebSocket behavior handles messages and read receipts.
- Current outgoing Redis and WebSocket routing is specialized for chat-message payloads, not arbitrary section events.
- Successful sends currently acknowledge the originating device with a plain client-message ID string; they do not provide complete server message metadata in a structured acknowledgment.
- The reply field exists in persistence models, but thread creation, retrieval, and UI-facing metadata are incomplete.
- Message history and sending now enforce their operation-specific participant policies. Future Markdown APIs and searches still require their own specified conversation permissions.

Here, authentication means verifying the JWT to identify the caller; conversation authorization means checking the caller's permission for the specific conversation operation. Active membership is relevant to some operations, but is not a universal requirement for every operation. The current differences are:

- `getMessagesHandler` extracts the authenticated caller's ID and passes it into the history service. The service rejects callers without a participant record, leaves active history unrestricted, and limits inactive history to the participant's recorded last-read sequence, inclusive. Repository queries apply the bound before ordering and limiting for latest, before, and after pagination. A null last-read value returns a successful empty result using the existing empty-result serialization.
- `InMssgHandler` checks sender state before allocating a sequence or inserting a message. Only `active` participants can send; absent, left, or removed participants return the existing message-module active-participant error. Rejections do not insert, advance the sequence counter, or publish. WebSocket transport behavior is retained: the error reaches the existing logging path and no success acknowledgment is sent.
- `CreateReadReceipt`, used by REST and WebSocket paths, requires an existing participant row but does not require its status to be `active`. The user confirmed that accepting delayed receipts for messages received while active is expected even after leaving or removal; this behavior is not an authorization defect.
- In contrast, the conversation participant-list service calls `VerifyConversationParticipant`, which checks both user/conversation identity and `status = 'active'`.

Outgoing recipient selection also filters active participants: after the message commits, `InMssgHandler` calls `FindConversationParticipants`, whose query requires `cp.status = 'active'`. It then selects only their non-revoked devices (`revoked_at IS NULL`), resolves device connections through Redis, and routes to local connected clients. This recipient filter is distinct from sender authorization, which now happens before insertion. Recipient membership is checked at selection time, not rechecked by each destination WebSocket writer.

The conversation repository now supplies participant status and nullable last-read sequence through `ConversationStore` within the message operation's transaction. It uses `FOR SHARE` to hold the participant state stable until the operation completes; concurrent status updates wait rather than changing membership between authorization and insertion. Domain decisions stay in the message service, SQL stays in repositories, HTTP translation stays in handlers, and access errors stay in `internal/message/errors.go`. No tables or migrations changed. Markdown operation permissions still need to be specified individually rather than assuming every operation requires current active membership.

## 4. Required Product Behavior

### 4.1 Sections

1. A conversation can be represented as a Markdown-like hierarchy of sections and messages.
2. A section has a Markdown-style heading and a unique identity.
3. Different sections can have the same header, including within one conversation.
4. Sections group consecutive messages; arbitrary scattered message membership is not the intended behavior.
5. Nested sections are supported.
6. Messages outside sections are allowed.
7. A section can be created before its topic has any messages.
8. Historical consecutive messages can be grouped into a section.
9. A section can extend only through the next consecutive sequence number. If it ends at 20, adding 21 is allowed; adding 23 while skipping 21–22 is not. Existing adjacent messages can be reassigned through boundary changes.
10. Sections are collapsible.
11. Users can search section headers and search message content within a section.
12. A message can reference multiple sections through `[[...]]` syntax.
13. Clicking a reference navigates to its target section.
14. Section structure, resolution, and search stay within the owning conversation.
15. The plus action is unavailable when a section has ended and another section has begun immediately after it. Backend mutations must enforce the corresponding boundary rules rather than relying on frontend visibility.
16. Plus buttons and dragging are future frontend controls. The current design must accommodate the backend operations that those controls will invoke, without implementing the controls themselves.
17. Empty sections are allowed, both before receiving any messages and after a boundary transfer removes their last message. Emptiness alone must not delete the section.
18. Removing a message from its containing hierarchy adjusts the immediate parent section first, then ancestors in rising order. Inserting into a different hierarchy adjusts the outermost affected section first, then inner sections in decreasing order toward the target.
19. Ordinary sends do not automatically extend any section, even one ending at the previous last message. Section-targeted sends must explicitly communicate that intention and the target section's identity.

### 4.2 Hashtags

1. A message can contain any number of hashtags.
2. Hashtag membership does not require consecutive messages.
3. Search returns messages mentioning the requested hashtag in the same conversation.
4. Exact spelling and case define identity: `#Go`, `#go`, and `#GO` differ.
5. An exact hashtag name is unique within a conversation; repeated mentions reuse that hashtag's identity.
6. Another conversation may independently contain a hashtag with the same name.

### 4.3 Replies

1. A reply is an independent message in the normal conversation timeline, like a WhatsApp reply, with its own message ID and conversation sequence number.
2. A message also exposes a collapsible thread of replies made to it; this is another view of the existing reply messages, not additional message rows.
3. Thread retrieval includes replies both inside and outside the target message's section. It must not filter solely by that section's boundaries.
4. A reply does not inherit section membership from the message it targets. Its membership follows its own chronological position and section boundaries.
5. A newly sent reply can extend a section only when the send explicitly targets that section and its sequence is immediately consecutive to the section's current last message, under the same rules as any other message. An ordinary reply send does not extend a section automatically. Replying to a message in A does not reopen A across intervening messages.
6. The thread presentation is collapsible, not a side panel.
7. Preserve each reply's exact `reply_to_message_id`, even when the target is itself a reply. Do not rewrite it to the oldest ancestor.
8. The thread for a message includes all its reply descendants, displayed together in one flat view rather than a recursively indented tree. A reply's own thread therefore includes its descendants as well; these are views over the same message rows.

Example: B replies to A, C replies to B, and D replies to A. A's thread includes B, C, and D; B's thread includes C. C retains B as its exact reply target. All four messages still appear independently in the chronological timeline and retain their own section membership.

Descendant inclusion is confirmed. Reply-search behavior, ordering within the flat view, thread pagination, and deleted-target behavior remain open. A cached root ID or separate thread table is not a confirmed requirement; first assess whether the existing parent-message relationship and a suitable index are sufficient for the required descendant queries.

## 5. Section Ownership and Membership

### 5.1 Sequence-Range Model Supported by Confirmed Behavior

The user proposed a separate sections table with the following fields, and has confirmed consecutive chronological boundaries as the membership behavior. The exact physical schema is still being designed:

| Conceptual field | Purpose |
| --- | --- |
| `id` | Stable section identity; unique even when headers repeat |
| `conversation_id` | Foreign key to the owning conversation |
| `header` | Display/search text; not unique |
| `start_sequence_no` | First covered message position |
| `end_sequence_no` | Last covered message position |

A parent-section relationship was recommended to express nesting. Final field names, empty-section representation, boundary inclusivity, indexes, and validation mechanisms are open. The UUID generation policy for new section IDs has not been selected.

In this model, membership is derived from boundaries rather than creating reference rows for each contained message. A section mention is a different relationship and must not be confused with membership.

### 5.2 Confirmed Consecutiveness and Boundary Operations

Section ranges follow chronological `sequence_no`, not UUID ordering or message creation timestamps. A plus action does not insert a message out of chronological order, and a drag action does not renumber messages.

For A covering sequences 10–20:

| Operation | Required result |
| --- | --- |
| Explicitly target A when sending new message 21, with no intervening section start | Consecutiveness permits extension; other applicable rules must also pass |
| Send ordinary message 21 while A ends at 20 | A remains unchanged; assigning 21 to A requires a later explicit boundary operation |
| Add a newly sent message with sequence 41 while 21–40 remain outside A | Reject; it is not the next sequence number |
| Move existing message 21, the first message of adjacent B, into A | Supported boundary transfer; A now ends at 21 and B must relinquish 21 at that sibling level |
| Move existing message 23 into A while A still ends at 20 | Reject; this skips sequence numbers |

The adjacent-section rule means that creating a new message for A and transferring an existing boundary message from B are distinct operations. The user explicitly allows the latter even when the former's plus action is unavailable. If B contains messages 21–40, transferring 21 would leave B starting at 22. If B contains only 21, B becomes empty and remains an existing section. The physical representation and placement of that empty section remain open.

The backend must validate these transitions against persisted state, including intervening sends or section changes. A boundary transfer involving two sibling sections must not leave inconsistent ranges; exact transaction/locking rules remain to be specified.

Consecutiveness is the literal next sequence number (`current end + 1`), not the next message ID. How deleted messages participate in selection and boundary validation remains open. Ordinary sends never automatically extend section boundaries. Consecutive eligibility alone is insufficient: a send must explicitly identify a target section, or the user must later invoke a boundary operation for the existing message. If ordinary message 21 remains outside A, a later targeted send of 22 cannot skip 21 to extend A.

The earlier alternative of separate document positions was proposed only to accommodate arbitrary insertion into historical sections. The user's confirmed behavior does not require that alternative. Preserve existing send sequences, pagination cursors, and read positions.

### 5.3 Confirmed Automatic Boundary Propagation

The user requires automatic shrink/expansion through nested sections, with different propagation directions for removal and insertion:

1. **Removal:** adjust the message's immediate containing section first, then its parent, and continue outward in rising hierarchy order.
2. **Insertion:** adjust the outermost affected section first, then work inward in decreasing hierarchy order until the intended target section is adjusted.
3. **Transfer:** reflect both source removal and destination insertion without changing the message's sequence or ID. Source sections that lose all content remain valid empty sections.

For the previously discussed example, A covers 10–20, adjacent B covers 21–40, and B's child C covers 21–25. Transferring message 21 from B/C into A shrinks C to 22–25, then B to 22–40, and expands A to end at 21. The user has chosen automatic adjustment rather than rejecting the transfer or requiring all of C to move with message 21. If C contained only message 21, C would remain as an empty child section.

These are logical operation orders, not permission to expose partially updated hierarchy state. The assistant recommends one transaction for all affected boundaries and publishing changes only after commit. The concrete locking strategy and SQL update order remain implementation decisions.

The direction rules do not yet define every affected range. The precise algorithm still needs to specify which descendant paths change on insertion, how ancestors retaining other content adjust, how shared ancestors are handled during transfer, and how empty sections retain their chronological anchor. Do not assume that every child acquires its parent's entire range or that unrelated sibling content can be silently removed. Literal sequence consecutiveness and valid containment remain constraints on the operation.

### 5.4 Recommended Structural Rules, Pending Agreement

- Nested sections must belong to the same conversation as their parent.
- A child's content must fit inside its parent under whichever membership model is selected.
- Siblings must not partially overlap; overlap through ancestry is intentional.
- Parent links must not form cycles.
- Historical grouping and boundary changes must preserve a valid hierarchy.
- Heading depth can follow nesting; the maximum depth and relationship to Markdown's six heading levels are unresolved.

Empty sections are confirmed as valid. Their representation, equal-boundary nested sections, moving whole sections, explicit section deletion, and conflicting boundary edits still need specification.

## 6. Section References

### 6.1 User-Proposed Association Table

The user proposes `section_references` with its own ID, a foreign key to the source message, and a foreign key to the target section. One message can have multiple reference rows without creating any additional message rows.

Example:

```text
messages
id    conversation_id    sequence_no    content
M1    C1                 42             See [[Design]] and [[Testing]]

section_references
id    message_id    section_id
R1    M1            S1
R2    M1            S2
```

Message M1 still appears once, at sequence 42. R1 and R2 do not have message sequence numbers.

### 6.2 Header Storage and Duplicate Headers

The user also proposed a foreign key to the referenced header. The assistant recommends storing only the section ID as the target relationship because headers may repeat and may change. A header is therefore not a suitable independent unique key.

If the product needs the original wording displayed in an old message, it can remain in message content or be stored as a plain reference label. That label would not be another foreign key. Whether renaming a section updates existing displayed link labels is open.

**Confirmed:** typing `[[header]]` presents matching sections from the current conversation; the user chooses one, and the reference persists the chosen section ID. Duplicate headers do not make the stored target ambiguous. Explicit path/ID typing was an earlier alternative and is not the chosen interaction. Exact autocomplete matching and the wire representation of a selected inline target remain open.

Mention storage granularity is also open: one relationship per distinct target per message versus one row per textual occurrence with occurrence metadata. Repeated identical headers targeting different sections require a representation that preserves the intended inline targets.

### 6.3 Optional Boolean

The user proposed `messages.has_section_references`. The assistant recommends omitting it initially because association rows already express existence, and a stored boolean must remain synchronized when references are created, edited, or removed. This is a recommendation, not an explicitly accepted decision.

A later performance measurement could justify such a field. The final choice is open; do not add it or declare it rejected without resolving the discussion.

## 7. Hashtag Data Model

The user's conversation-local name uniqueness requirement supports a hashtag dictionary and an association table. The assistant recommended:

```text
hashtags
id
conversation_id
name

message_hashtags
id
message_id
hashtag_id
```

Recommended constraints and access paths:

- Unique exact `(conversation_id, name)` on hashtags.
- Unique `(message_id, hashtag_id)` on associations, if occurrence counts are not needed.
- An index beginning with `hashtag_id` to retrieve associated messages.
- Conversation checks so an association cannot link a message to another conversation's hashtag.
- Exact, case-sensitive name matching; no lowercasing or stemming for hashtag identity.

The physical schema has not been approved. A dictionary was initially described as optional before the user clarified conversation-local hashtag identity; the dictionary model is now the recommendation for that clarified requirement.

Open details include hashtag token syntax, Unicode handling, punctuation, escaped hashtags, code spans/blocks, whether the stored name includes `#`, and whether hashtags can be managed independently from message text. Message editing/deletion must eventually have defined association-update behavior.

## 8. Storage Growth and Message Retrieval

The user's motivation for arrays is to avoid increasing row counts in an already fast-growing messages table. The design must preserve the distinction between message rows and association rows.

- Separate references add storage and index writes, but do not enlarge the number of rows in `messages`.
- One message keeps one ID and one conversation sequence number regardless of reference or hashtag count.
- A history query can fetch the message page first, then fetch associations for all page message IDs in one batched query per association type.
- Build API arrays from those association results. Database normalization does not require returning duplicate message objects over the API.
- Avoid an unaggregated join that multiplies message rows and corrupts pagination or limits.
- Row count alone does not establish whether arrays or association tables perform better; measurements should reflect actual queries and indexes.

The assistant recommends association rows for stable entity relationships: normal foreign keys do not validate each element of an ID array, and rows simplify backlinks, uniqueness, deletion handling, and relationship metadata. Arrays remain suitable for request/response payloads, caches, or small lists of values owned by one record. No blanket prohibition on arrays was agreed.

Reference: [PostgreSQL array guidance](https://www.postgresql.org/docs/17/arrays.html).

## 9. Backend and Realtime Direction

### 9.1 Database Choice

The assistant recommends keeping PostgreSQL. The requirements currently involve conversation-local trees, ordered messages, direct links, and hashtag lookup. A graph database has not been selected, and no workload demonstrating a need for one has been identified.

### 9.2 Proposed Backend Capabilities

The feature will need capabilities for creating and listing sections, fetching hierarchy and section content, grouping historical messages, changing consecutive boundaries with automatic hierarchical propagation, transferring an adjacent boundary message between sections, selecting/resolving section references, searching headers/content/hashtags, and creating/fetching replies. Normal sends and replies share the conversation's existing message sequence. Thread retrieval is an additional view over those messages, includes recursive descendants, and can span sections. Exact routes, payloads, permissions, and pagination contracts remain open.

**Confirmed send behavior:** the incoming message must distinguish an ordinary send from an explicit section-targeted send and identify the target section for the latter. The assistant recommends an optional `section_id`: absent means no section extension; present identifies the target and expresses intent. The exact field name and null/omission contract are not frozen. The message's reply target is independent of this section target. Section-targeted insertion and boundary extension should be committed together; a concurrent intervening send must not cause the backend to bridge a sequence gap silently. Exact conflict responses remain open.

Message content can remain text while structural relationships live separately. No decision has been made to change `messages.content` to JSONB.

### 9.3 Candidate Socket Events

The assistant proposed a general event envelope carrying `type`, `conversation_id`, and event `data`, replacing the assumption that every outgoing routed payload is a chat message. The following names are candidates, not a frozen protocol:

| Candidate event | Intended meaning |
| --- | --- |
| `message.created` | A new message, with relevant section placement, references, hashtags, and optional thread identity |
| `section.created` | An empty section or a section wrapping historical content was created |
| `section.updated` | Section metadata such as the header changed |
| `section.content_updated` | Boundaries, membership, order, or nesting changed |
| `section.deleted` | A section was removed; content handling still needs definition |

A reply could use `message.created` with its exact `reply_to_message_id` rather than a separate event type. It must remain visible as an independent timeline message and also be attachable to the target's and ancestors' descendant-thread views, regardless of section membership. Thread aggregation is now confirmed to include all descendants; whether to include cached ancestor/root metadata in storage or events remains open. No final choice has been made about command transport, acknowledgments, event identifiers, versions, or dedicated reply events. A section-targeted send or boundary transfer must communicate all resulting section changes, including affected nested sections and ancestors, so clients can reconcile the whole hierarchy; the exact event shape is open.

Recommended behavior is to commit persistent changes before broadcasting them. Message-only sequence catch-up cannot recover a historical section rename or boundary change, so reconnect behavior must also recover section structure. Reloading structure through REST is the initial recommendation; a durable structural event log has not been requested or selected.

Collapsing/expanding a section or thread can remain local frontend state unless the user wants per-user preferences synchronized across devices. Shared collapse-state broadcasts have not been requested.

## 10. Unresolved Decisions and Next Discussion

The initial behavior questions and the narrower follow-ups are answered: consecutive ranges, selected reference IDs, independent timeline replies, permitted empty sections, directional automatic boundary propagation, explicit section targeting, and flat views of all reply descendants. Resolve the following before producing an implementation-ready plan:

- Empty-section representation and boundary conventions.
- Empty-section chronological placement, first-message assignment, and how a section is explicitly ended without starting another section.
- Exact affected-path boundary algorithms, shared ancestors during transfer, and preserving other content while propagating adjustments.
- Flat descendant-thread ordering, pagination, deleted-message handling, and query/index strategy.
- Boundary conflicts, nesting limits, and section deletion/content preservation.
- Who can create, rename, resize, move, or delete sections in a conversation.
- Reference labels after rename, unknown references, target deletion, and message edits.
- Whether to include the proposed `has_section_references` boolean.
- Hashtag syntax and association lifecycle.
- Search semantics: exact versus partial header matching, content matching, descendant inclusion, reply inclusion, and pagination.
- Canonical schema, database constraints/indexes, and transaction/concurrency rules.
- APIs, socket commands/events, multi-device updates, and reconnect recovery.
- Whether actual Markdown export is required; the user requested representability and Markdown mode, but an export endpoint and serialization format have not been specified.
- Migration strategy for this feature, compatibility expectations, and focused acceptance tests.

## 11. Change History

### 2026-10-20 — Message History and Send Permissions Implemented

- Implemented the user's requested history policy: no participant record means HTTP 403; active participants retain existing behavior; left/removed participants receive only messages through their current last-read sequence, inclusive. A null last-read value produces empty history.
- Applied inactive-history cutoffs before pagination and limits in all three repository queries. Preserved response fields, ascending ordering, and existing cursor semantics.
- Implemented active-sender checks before sequence allocation and insertion. Kept WebSocket framing and read-receipt behavior unchanged.
- Added a transaction-aware conversation repository lookup through `ConversationStore`, using a shared participant-state model and `FOR SHARE`. Kept policy in message services, SQL in repositories, transport handling in handlers, and domain errors in the module's errors file.
- Validated the policy in a disposable PostgreSQL schema, including active/inactive pagination, nonparticipant rejection, send rejection without sequence changes or publication, active UUIDv7 sends routed through Redis, delayed receipts, and participant-row locking.
- Updated the reusable context and baseline from the earlier observed gaps to the implemented behavior. No Markdown chat implementation or schema change was made.

### 2026-10-20 — Delayed Read Receipts Are Expected

- The user clarified that a read receipt for a message received during active membership remains valid after the reader leaves or is removed and should be recorded.
- Corrected the earlier framing that treated missing active-status enforcement on receipts as a defect. Do not introduce an active-only restriction for this case.
- Distinguished operation-specific permissions from a blanket active-membership requirement. Updated the baseline and reusable context accordingly; no backend code changed.

### 2026-10-20 — Empty Sections, Boundary Propagation, Explicit Sends, and Descendant Threads

- Confirmed that empty sections remain valid, including when transfers remove their final messages; do not delete them merely because they become empty.
- Confirmed source-side propagation from the immediate containing section outward through ancestors, and destination-side propagation from the outermost affected section inward toward the target.
- Recorded automatic shrinking in the adjacent B/C example, including retention of an emptied child section.
- Confirmed that ordinary sends never extend sections automatically. Section-targeted incoming messages must carry explicit intent and section identity; optional `section_id` is the recommended minimal representation, pending final protocol naming.
- Confirmed exact `reply_to_message_id` preservation and a flat thread view containing all descendants, while replies remain independent timeline messages and retain positional section membership.
- Updated the context summary, requirements, backend/event direction, and open questions to remove superseded uncertainties. Kept exact storage, concurrency, and edge-case algorithms open rather than inventing decisions.
- No Markdown chat backend implementation was performed.

### 2026-10-19 — Behavior Questions Answered

- Confirmed literal sequence consecutiveness for section extension and historical boundary transfers. Moving 21 after an end of 20 is permitted; skipping to 23 is not.
- Recorded that plus controls are unavailable after an adjacent new section begins, while transferring that next existing boundary message remains supported.
- Distinguished backend boundary operations from the future frontend plus/drag controls; message IDs and chronological sequences remain unchanged.
- Recorded that the chosen behavior supports the range model and removes the original motivation for separate document ordering.
- Confirmed matching-section selection for `[[header]]` and persistence of the chosen section ID.
- Confirmed replies as independent chronological messages plus a collapsible thread view, without automatic inheritance of the target's section. Threads include replies across section boundaries.
- Replaced the formerly deferred questions in the context summary and open-decision list, retaining the narrower unresolved issues rather than assuming their answers.
- No Markdown chat backend implementation was performed.

### 2026-10-19 — Initial Design Record

- Created this document at the user's explicit request, including a reusable context summary and maintenance instructions.
- Recorded conversation-scoped sections, nested consecutive grouping, historical grouping, references, hashtags, and collapsible replies as requirements.
- Recorded the user's range-based section and separate-reference-table proposals.
- Clarified that references and hashtag associations never duplicate message rows or consume conversation sequence numbers.
- Recorded conversation-local exact hashtag identity and the resulting dictionary-table recommendation.
- Distinguished recommendations concerning header references and the optional boolean from finalized decisions.
- Preserved the three deferred behavior questions and the unresolved ordering tradeoff.
- Recorded candidate backend/event capabilities without treating them as finalized APIs.
- No Markdown chat schema or backend implementation was performed as part of creating this document.
