# Markdown Chat Backend Design

Last updated: 2026-10-23

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

The user is concerned about the growth rate of the messages table and preserving one message row per unique conversation sequence number. Association rows must live in separate tables: they must never duplicate messages or consume message sequence numbers. The user's proposed structure is a sections table with conversation ID and start/end message sequence numbers, a separate `section_references` table linking messages to sections, and analogous tables for hashtags. The user has explicitly accepted omitting the earlier proposed `has_section_references` boolean. The latest inline-reference proposal stores the selected target ID and display label together in message content, with derived reference rows for lookup, as described below and in section 6.4.

The user has now answered the three initial behavior questions:

1. Section extension requires consecutive message sequence numbers. If A ends at 20, message 21 can be included next; 23 cannot be included while 21–22 remain outside A. The plus action is unavailable when a section has ended and another has begun immediately after it. A newly sent message cannot be inserted visually into an earlier section across intervening messages. Moving the first message of adjacent B into A changes section boundaries, not the message's identity or chronological position. The frontend controls come later; the backend must support and validate their effects now in the design.
2. Typing `[[header]]` shows matching sections, the user selects one, and the reference stores the chosen section ID. Headers may repeat.
3. Replies have their own message rows and conversation sequence numbers and appear in the normal timeline as well as in a collapsible reply thread. Preserve the exact `reply_to_message_id`, including when replying to a reply. A message's thread includes all descendants, flattened into one view, regardless of their sections. Reply targets do not confer section membership.

On 2026-10-20 the user confirmed that empty sections are allowed, including after losing their final message. Nested boundaries adjust automatically: removal updates the immediate owning section first and proceeds outward through ancestors; insertion updates the outermost affected section first and proceeds inward toward the target. Ordinary sends never automatically extend a section. Sending through the future plus control must explicitly identify the target section in the incoming message; alternatively an ordinary sent message can later be assigned by a valid consecutive boundary operation. An optional `section_id` is the assistant's recommended wire representation; a separate boolean is not required to distinguish targeted and ordinary sends.

These decisions support sequence-range section membership and remove the need for a separate document-order model for the requested behavior. Empty sections have both sequence boundaries null, and section creation chronology is retained by `created_at`. Exact nested-boundary algorithms and protocol details still need specification before implementation. The assistant recommended PostgreSQL, separate relational association tables, and a distinction between section membership and section mentions. No graph database or JSONB message-content change has been agreed.

A preceding, separate task implemented UUIDv7 for new server-generated message IDs using the existing `github.com/google/uuid` dependency. Go and PostgreSQL continue to use `uuid.UUID` and `UUID`. The initial migration and standalone schema no longer generate UUIDv4 message IDs by default; seed messages have explicit UUIDv7 IDs. Existing-ID conversion was unnecessary because the application is not yet in production and can start with a fresh database.

The user requires this detailed design document, including this reusable context summary, to be kept current as the discussion evolves. The latest update records permitted empty sections, automatic directional boundary propagation, explicit section targeting for sends, and exact reply targets with flat recursive-descendant threads. Continue the design conversation without treating remaining unanswered questions as settled or implementing the feature prematurely.

The user also requires a separate, continuously maintained frontend specification covering all existing APIs and end-to-end flows as well as the evolving Markdown experience. That document is [markdown_chat_frontend_design.md](markdown_chat_frontend_design.md). Keep both documents synchronized as decisions or backend contracts change, preserving the distinction between implemented APIs and confirmed future behavior with pending contracts.

Read-receipt policy clarification: a delayed receipt for a message received while the user was an active participant remains valid after the user leaves or is removed. Recording that read is expected behavior. Do not classify the absence of an active-status check in the receipt path as a defect or add such a restriction as part of this design.

Message access policy is now implemented: history requires a participant record, returning HTTP 403 for nonparticipants. Active participants retain the existing history behavior. Left/removed participants can fetch only through their current `last_read_mssg_seq`, inclusive, with the bound applied before pagination and limits. A missing last-read sequence means empty history. Sending requires active membership checked before sequence allocation and insertion. Accepted delayed receipts can advance an inactive participant's history cutoff. Services own these decisions, repositories own SQL, handlers own transport responses, and domain errors remain in the message module's errors file. No Markdown feature or schema change was needed for this access-policy update.

On 2026-10-23 the user requested a feasibility assessment of an idea, explicitly not a proposal or adoption decision: one canonical conversation file in MinIO, referenced by a PostgreSQL conversation object key, with messages as multiline blocks retaining existing metadata, and Elasticsearch for keyword search within/across conversations. The assessment is in section 9.4. No storage replacement or new cross-conversation Markdown scope has been agreed. The assistant considers it technically feasible, particularly for bounded, document-oriented workloads, but recommends retaining structured primary storage for live messages and treating Markdown objects as generated snapshots unless measured requirements favor a file-first model. Elasticsearch is independently usable with either storage choice. No MinIO/Elasticsearch code, dependency, or infrastructure was added.

The user subsequently asked whether a collaborative-document model using CRDTs, browser IndexedDB, real-time synchronization, and PostgreSQL persistence is more feasible. Section 9.5 evaluates this as another exploratory direction, not a decision to replace existing storage or message semantics. It is a credible architecture for collaborative editing and offline convergence. A JSONB document alone does not retain CRDT merge state; for example Yjs requires native binary state/updates, which PostgreSQL can store as `BYTEA`, with JSONB optionally serving as a rendered/query projection. The server must still decide message sequences, admission of offline sends into consecutive sections, and operation-specific permissions. Existing section invariants do not follow automatically from generic CRDT convergence. No CRDT library, provider, server runtime, or schema has been selected or implemented.

After these assessments, the user clarified that the primary motivation for the file approach was drastically reducing database rows and tables and concluded that the approach appears to introduce more challenges than benefits. Treat file-first replacement as set aside for now. This does not approve CRDT adoption, generated snapshots, Elasticsearch, or any other replacement architecture. Continue refining the existing relational Markdown-chat design, keeping schema size and association storage proportional to actual requirements; fewer rows alone is not a demonstrated performance improvement.

The user has now explicitly scrapped the file-based conversation idea and returned to planning the relational database modifications. The agreed working inventory is four new tables: `sections`, `section_references`, `hashtags`, and `message_hashtags`. Their conversation IDs keep structure, mentions, and hashtag identity scoped to the conversation; exact columns/constraints remain to be planned. A section also needs its header. The user confirmed nullable `start_sequence_no` and `end_sequence_no`, both null for an empty section, with `created_at` preserving section creation chronology. All four new tables must include both `created_at` and `updated_at`. An older empty section cannot receive a new tail message after a later section has begun: for A = 10–20, B = empty, C = 21–30, in creation order, new message 31 cannot be assigned to B. Existing `messages.reply_to_message_id` supports reply relationships without requiring a new threads table. No database migration has yet been authorized for these Markdown tables; the next task is database design planning. Section 9.6 records the inventory and remaining schema decisions.

The user confirmed self-contained inline links with standard Markdown ordering: type `[[parent section#child section header]]`, select a matching section, then send/store `[Parent section#child section header](referenced-section-id)` in `messages.content`. This supersedes the initial reversed `(label)[id]` example. The exact target convention/escaping still needs specification. The backend derives `section_references` from these links for reverse lookup and finding messages mentioning a searched section. Renderers resolve each occurrence from its embedded ID without fetching association rows to pair labels with IDs. Reference rows are derived search relationships, not a second independently editable source for inline positions. On section rename the frontend asks whether to update reference labels in the user's older messages: yes updates persisted message content; no preserves old labels and unchanged target IDs. Working author scope is the acting user's own messages; broadening that scope has not been authorized. One association per distinct message/section pair is now confirmed. Customized-label handling, parent-path propagation, and the bulk-update API/transaction contract remain open.

The user has also confirmed one `message_hashtags` row per distinct message/hashtag pair even when that literal repeats in the message. Hashtags have no click action; users find tagged messages by explicitly searching the exact hashtag string. Parent and child sections may have identical boundaries. Creation of overlapping sections is rejected; valid ancestor/descendant containment, including equal ranges, remains permitted. These decisions do not alter the established automatic adjustment behavior for explicit boundary transfers.

During physical database planning, the user confirmed that nonempty sections are placed by message ranges even when historical grouping happens later; `created_at` records creation order and does not make a newly grouped old range the conversation tail. Deleting a section removes its heading, promotes its direct children to its former parent/root, and preserves messages; old Markdown links remain visibly unavailable. Section 9.7 now contains a complete proposed physical schema, indexes, foreign-key behavior, service/SQL responsibility split, and migration/validation plan. Candidate DDL was executed successfully against PostgreSQL 17 in an isolated transaction and rolled back. No application schema or migration file has been changed for Markdown chat. Engineering defaults in 9.7 remain a reviewable proposal rather than user-confirmed product requirements.

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
20. An empty section stores null in both `start_sequence_no` and `end_sequence_no`; its creation chronology is preserved by `created_at` even without message boundaries.
21. Being empty does not exempt a section from tail-append eligibility. With A = 10–20, B = empty, and C = 21–30 created in that order, new message 31 cannot be inserted into B because B is not the last section.

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

The user confirmed `sections` as part of the four-table working inventory and confirmed consecutive chronological boundaries as the membership behavior. The exact physical schema is still being designed:

| Conceptual field | Purpose |
| --- | --- |
| `id` | Stable section identity; unique even when headers repeat |
| `conversation_id` | Foreign key to the owning conversation |
| `header` | Display/search text; not unique |
| `parent_section_id` | Parent in the section hierarchy; null for a top-level section |
| `start_sequence_no` | First covered message position |
| `end_sequence_no` | Last covered message position |

A parent-section relationship expresses nesting. Both sequence-boundary fields are nullable; an empty section has both null. `created_at` preserves section creation chronology. Exact constraints, indexes, and validation mechanisms remain to be designed. The UUID generation policy for new section IDs has not been selected.

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

The adjacent-section rule means that creating a new message for A and transferring an existing boundary message from B are distinct operations. The user explicitly allows the latter even when the former's plus action is unavailable. If B contains messages 21–40, transferring 21 would leave B starting at 22. If B contains only 21, B becomes empty and remains an existing section, with both boundaries set to null and its original `created_at` retained.

For sections created in the order A (10–20), B (empty), C (21–30), a new message with sequence 31 cannot target B. Null boundaries do not make any old section eligible for a new tail message. The backend must check section chronology as well as sequence adjacency. Historical message transfers remain distinct from new-message insertion. The exact interpretation of last-section eligibility across ancestor/child relationships and timestamp ties still needs to be specified during schema/API planning; do not introduce a separate position column merely to represent an empty section without a demonstrated requirement.

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

The direction rules do not yet define every affected range. The precise algorithm still needs to specify which descendant paths change on insertion, how ancestors retaining other content adjust, and how shared ancestors are handled during transfer. Empty sections retain their creation chronology through `created_at`. Do not assume that every child acquires its parent's entire range or that unrelated sibling content can be silently removed. Literal sequence consecutiveness and valid containment remain constraints on the operation.

### 5.4 Structural Constraints and Remaining Recommendations

- Nested sections must belong to the same conversation as their parent.
- A child's content must fit inside its parent under whichever membership model is selected.
- Sibling ranges must not overlap, including identical sibling ranges. Ancestor/descendant containment is intentional; identical parent and child boundaries are explicitly allowed.
- Parent links must not form cycles.
- Historical grouping and boundary changes must preserve a valid hierarchy.
- Heading depth can follow nesting; the maximum depth and relationship to Markdown's six heading levels are unresolved.

Empty sections are confirmed as valid and use paired null sequence boundaries. A creation request whose range overlaps an existing section outside a valid ancestor/descendant relationship is rejected rather than silently shrinking or moving existing sections. This rule is separate from the confirmed automatic propagation for explicit boundary transfers. Equal-boundary parent/child sections are explicitly permitted. Moving whole sections, explicit section deletion, and conflicting boundary edits still need specification.

## 6. Section References

### 6.1 User-Proposed Association Table

The user confirmed `section_references` in the working inventory with its own ID, `conversation_id`, a foreign key to the source message, and a foreign key to the target section. One message can have multiple reference rows for different targets without creating any additional message rows. Repeated links to the same section produce a single association for that message/section pair; individual occurrences remain in message content. Exact foreign keys and indexes remain to be designed.

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

The stored display label remains in message content and is not a foreign key. **Confirmed rename behavior:** ask the acting user whether they want reference headers updated in their older messages. If yes, persistently update the affected message content; if no, preserve existing labels. Navigation still uses unchanged section IDs. The working interpretation is messages authored by the acting user; do not assume permission to rewrite others' content. Exact bulk-update contracts and customized-label/path rules remain open.

**Confirmed:** typing `[[header]]` presents matching sections from the current conversation; the user chooses one, and the reference persists the chosen section ID. Duplicate headers do not make the stored target ambiguous. A parent/child label such as `[[parent section#child section header]]` can be used as input to this selection UI. Typing the target ID manually is not required. Standard `[label](target)` link ordering is selected; exact autocomplete matching, target encoding, and escaping remain open. Section 6.4 records the self-contained wire/content representation.

**Confirmed association granularity:** one relationship per distinct target section per message, not per textual occurrence. The self-contained content resolves inline occurrence-to-target pairing without a separate occurrence-offset structure; repeated identical labels can embed different IDs directly. Different target IDs produce different association rows even if their labels match.

### 6.3 Reference-Existence Boolean Omitted

**Confirmed:** omit `messages.has_section_references`; the user accepted deriving reference existence from the association table. No synchronization flag or replacement count column is needed. The earlier boolean proposal is superseded.

### 6.4 Self-Contained Inline Links and Derived Reference Rows

**Confirmed approach:** embed each selected section's display label and ID together in the stored message content using standard Markdown `[label](target)` ordering. Derive the searchable association table from that content. Exact target convention and escaping still need specification.

Confirmed authoring flow:

1. The user types text such as `lorem ipsum [[parent section#child section header]]`.
2. The frontend shows matching sections within the current conversation.
3. Selecting a result binds the individual link occurrence to the chosen section ID.
4. The frontend serializes the link with both display label and target ID, sends the content over the socket, and the backend stores it in that form.
5. The backend extracts references from the content and creates the corresponding `section_references` rows.

The selected stored form is:

```text
lorem ipsum [Parent section#child section header](referenced-section-id) dolor sit ...
```

The initial `(label)[id]` example was superseded by the user's explicit selection of standard Markdown ordering. An existing Markdown parser can represent the link, with application-specific interpretation for the selected section ID. Escaping rules, target convention, and recognition of actual links versus literal/code text belong in the eventual serialization contract.

Benefits and responsibilities:

- Message rendering can show only the label while resolving clicks from the embedded ID. No separate association fetch is required solely to map each displayed occurrence to a target.
- Identical labels within the same message can point to different section IDs unambiguously. Multiple occurrences pointing to the same ID remain representable.
- `section_references` serves reverse lookup/search: identify matching sections by name, then retrieve messages referring to those section IDs. This is distinct from the links' rendering role.
- `messages.content` is the canonical representation of inline occurrences; reference rows are derived relationships. They must not be independently edited into disagreement with the content.
- The assistant recommends creating/updating the message content and derived associations within the same transaction, keeping policy in the service and database writes in repositories. Exact edit behavior is not implemented yet.
- Target sections must be resolved/validated in the message's conversation, matching the confirmed scope. A display label is not another foreign key or an authoritative replacement for `sections.header`.
- Stored label text is preserved unless the user opts into updating their older messages during the section-rename flow. Yes must update persisted content, not just substitute the display at render time. Match affected links by target ID, not duplicate-prone header text. Search by current section name and searching historical label text are different queries and must not be conflated.

The user accepted one association per distinct `(message_id, referenced_section_id)`. Enforce that pair's uniqueness in the schema; repeated mentions remain present in content without duplicating association rows. No offset, occurrence-ID, or extra reference-label table is required just to render links.

Renaming may affect links in messages outside the currently loaded history page. The backend must provide an operation covering the intended persisted scope; looping over visible client messages is insufficient. No section-rename, message-edit, or bulk-label-update endpoint exists yet. Exact atomicity versus batching, failure/retry responses, and update events remain to be designed. The frontend flow and API dependencies are maintained in [markdown_chat_frontend_design.md](markdown_chat_frontend_design.md), section 13. The opt-in prompt is a confirmed exception to preserving old label snapshots, not a global rule to rewrite every participant's messages automatically.

## 7. Hashtag Data Model

The user's conversation-local name uniqueness requirement supports a hashtag dictionary and an association table. The assistant recommended:

```text
hashtags
id
conversation_id
name

message_hashtags
id
conversation_id
message_id
hashtag_id
```

Confirmed uniqueness rules and recommended access paths:

- Unique exact `(conversation_id, name)` on hashtags.
- Unique `(message_id, hashtag_id)` on associations, regardless of how many times the literal appears in the message.
- An index beginning with `hashtag_id` to retrieve associated messages.
- Conversation checks so an association cannot link a message to another conversation's hashtag.
- Exact, case-sensitive name matching; no lowercasing or stemming for hashtag identity.

The user has confirmed `hashtags` and `message_hashtags` in the four-table working inventory, explicitly including `conversation_id` on both. The physical schema has not been approved. A dictionary was initially described as optional before the user clarified conversation-local hashtag identity; the dictionary model is now part of the working inventory.

**Confirmed frontend behavior:** clicking a hashtag does nothing. Hashtags do not navigate like section links and do not initiate search on click. Users explicitly search by the exact hashtag string to retrieve all associated messages within the same conversation. Association rows support set membership, not per-occurrence counts.

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

### 9.4 Evaluated Idea: One Conversation File in MinIO with Elasticsearch

**Status: exploratory assessment only, not an adopted architecture.** The user explicitly described this as an idea to assess rather than a proposal to implement. Existing requirements and the current PostgreSQL implementation remain in effect.

**Subsequent user assessment:** the motivation was drastically reducing the number of rows and tables. After considering the alternatives, the user considers the file approach to have more challenges than benefits. Set it aside for now; the analysis below is retained as context, not an active implementation plan.

**Latest decision:** the user explicitly scrapped the file-based conversation idea and resumed the relational schema design. Keep this assessment as historical reasoning, not an implementation direction.

#### Idea and Potential Benefits

The conversation's content would be a single object in MinIO. PostgreSQL would retain an object-key reference for that content. Each message would be a block of consecutive lines carrying its message ID, sender ID, sequence, timestamps, reply target, and other existing metadata. Elasticsearch would support keyword search within a conversation and across conversations.

This can provide a portable, document-shaped representation, natural heading order, empty headings, straightforward full-document export, and fewer PostgreSQL message/association rows if those records are actually removed. It could be a reasonable source of truth for small or bounded conversations with low write contention and document-oriented access. Feasibility does not establish that it is simpler or cheaper for this application's workload.

The comparison assumes participant/session authorization remains structured; moving content to an object does not mean abandoning membership or last-read state. The exact division of metadata between PostgreSQL and files has not been selected.

#### Object Write Semantics and Concurrency

A standard object upload replaces the object rather than editing a selected message block in place. A naive read-modify-upload implementation would transfer the entire conversation on each send or historical edit. If messages have roughly equal size and every send uploads all prior content, cumulative uploaded bytes grow quadratically with the number of messages. This describes the naive replacement implementation, not every possible object-backed system or physical disk write amplification.

Do not claim that all MinIO deployments lack append support: current MinIO **AIStor** documents append operations specifically in S3 Express mode, using the current byte length as the append offset. This capability must be verified against the actual edition/version/configuration selected for deployment. The documented mode also changes available features, including disabling versioning and replication APIs. Append does not provide arbitrary in-place editing of historical headings or message blocks. [MinIO AIStor S3 Express documentation](https://docs.min.io/aistor/reference/aistor-server/s3-express/)

Two backends modifying the same object revision can lose one another's updates if they blindly overwrite it. The architecture must serialize a conversation's mutations or detect conflicts and retry. Conditional object writes are one relevant mechanism; exact MinIO support must be validated rather than inferred from AWS support. PostgreSQL already briefly serializes sequence allocation for a conversation, so serialization itself is not unique to the file idea; the new concern is how much file/network work is placed on that serialized path. [S3 conditional write semantics](https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html)

PostgreSQL state and an object upload do not share the backend's current PostgreSQL transaction. A file-first design must define when a send is committed and acknowledged and how retries avoid duplicate or missing messages. One possible approach is immutable object revisions with a PostgreSQL pointer update committing the authoritative revision; this is an example to evaluate, not a selected mechanism. Whole-object publication can be atomic without making the PostgreSQL/object/search update sequence one atomic transaction.

#### File Structure Does Not Remove Message Semantics

A line-oriented document still needs stable block boundaries, unique message/section IDs, correct sequence allocation, retry deduplication, reply links, selected section-reference targets, and nested section-boundary rules. Headings inside an ordinary message body must be distinguishable from structural section headings. Duplicate header names still require ID-backed references. Message edits can shift line and byte offsets, so neither is a stable replacement for identity.

An extended Markdown format could store metadata in explicit block wrappers or comments; JSON/another structured representation could instead be rendered as Markdown. No canonical syntax, metadata schema, or choice between these representations has been made. Ordinary Markdown alone is not a complete serialization contract for all existing message fields.

Historical regrouping can be expressed by moving heading markers, but a transfer remains a validated structural operation. Automatic nested-boundary propagation and consecutive message order remain required. A file format simplifies document serialization, not necessarily those business rules.

#### Retrieval, Threads, and Existing Access Policy

History must still fetch bounded pages by sequence. Thread lookup must find all descendants while preserving exact reply targets. Inactive participants must still receive only through their current last-read sequence. A single file without a lookup structure may require scanning/parsing substantial content for these operations.

Byte-range reads can reduce transfer if the application maintains a suitable version-aware block/offset index; they do not intrinsically locate sequence numbers or reply descendants. Adding such indexes or caches is feasible but is additional architecture, not an automatic consequence of storing the file in MinIO. Offsets can need rebuilding after historical edits.

A full canonical object must not be handed directly to an inactive participant when it includes content beyond their allowed cutoff. The backend can parse/filter it or serve an authorized projection. Object-key possession alone does not encode per-message visibility.

#### Elasticsearch Is an Independent Choice

Elasticsearch can index either PostgreSQL-backed messages or file-backed blocks. It does not require a move to MinIO and does not automatically parse the application's Markdown format; an ingestion path must construct and update searchable documents.

The assistant recommends one search document per message/block, with conversation ID, message ID, sequence, text, and relevant structured fields, if Elasticsearch is adopted. One growing search document per conversation makes per-message result pagination and updates less convenient. Reducing PostgreSQL row counts would not eliminate per-message indexing/storage in Elasticsearch.

Search visibility is normally near-real-time and governed by refresh behavior. The index must be synchronized and rebuildable from canonical content; if it also serves timeline/thread queries, its freshness and availability become part of those read-path requirements. [Elasticsearch refresh behavior](https://www.elastic.co/docs/reference/elasticsearch/rest-apis/refresh-parameter)

Exact case-sensitive hashtags require explicit keyword mapping without lowercasing normalization, distinct from analyzed full-text message search. Search results, snippets, and aggregations must respect caller-specific conversation and sequence visibility. Cross-conversation keyword search was mentioned in this idea; it does not authorize cross-conversation section links, hashtag identities, or Markdown scope, and no search API expansion has yet been adopted. [Elasticsearch exact-match filtering](https://www.elastic.co/guide/en/elasticsearch/reference/current/full-text-filter-tutorial.html)

#### Assessment and Alternatives

| Option | Assessment |
| --- | --- |
| One frequently replaced canonical object per live conversation | Feasible; simple document representation, but growing write/read work and conversation-level conflicts require evaluation |
| Append-capable object with structural edit records | Can reduce append costs on a supported deployment, but requires event replay, indexes, and/or compaction rather than maintaining only a directly editable Markdown document |
| PostgreSQL canonical messages/structure with generated MinIO Markdown snapshots | Assistant's current recommendation: preserves transactional live operations while providing a single-file representation; snapshot freshness is explicit |
| Immutable/chunked historical objects with an index or manifest | Possible future history-storage direction if measurements justify it; departs from the literal single-physical-file idea |

No option is adopted by this assessment. If file-first storage is explored further, evaluate expected maximum conversation size, peak writes per conversation, historical edit frequency, page/thread latency, snapshot/search freshness expectations, and the actual object-store capabilities. A small prototype can compare transferred bytes, latency, conflict/retry rates, and recovery behavior against the existing storage model. Do not introduce these systems or replace the message schema without a separate decision.

### 9.5 Evaluated Idea: CRDT Collaborative Conversations with PostgreSQL Persistence

**Status: exploratory assessment only.** The user is asking whether treating conversations as collaboratively edited documents is more feasible than the earlier object-file approach. MinIO is not a requirement. No decision has been made to replace message rows, adopt a CRDT engine, introduce unrestricted shared editing, or change existing chronological semantics.

#### Feasibility and What the Approach Provides

This is an established architectural pattern: clients maintain a collaborative document, persist local state in IndexedDB, exchange CRDT updates through a network provider, and durably persist server state. It can support immediate local edits, merging concurrent operations, and reconnect synchronization. PostgreSQL is a feasible persistence backend, including for encoded snapshots and update records. Automerge describes a storage-adapter design supporting stores such as PostgreSQL, snapshots, and incremental changes. [Automerge storage](https://automerge.org/docs/reference/under-the-hood/storage/)

CRDTs are particularly relevant when multiple users edit the same message body, heading text, or structured document concurrently, including offline. A shared structured document can represent messages and sections as stable-ID blocks instead of identifying them by mutable line offsets. Markdown can remain an export/rendering form. Exact CRDT data types and editor binding are not selected.

For ordinary immutable message sends, a local pending-message queue with server acceptance can also provide offline composition; CRDTs are not required merely to queue sends or render sections. The benefit depends on which parts participants are actually allowed to co-edit.

#### JSONB Is Not Automatically the CRDT's Durable State

PostgreSQL can store a JSONB conversation document, but a JSON representation of visible content usually lacks native CRDT identity and merge metadata. For Yjs specifically, persist its binary state/updates rather than reconstructing a fresh document from exported JSON on each load. Hocuspocus explicitly warns that JSON-only reconstruction breaks merging and can duplicate content. `BYTEA` is a suitable PostgreSQL representation for those binary bytes; JSONB can additionally store a derived structured projection. Encoding binary state inside JSON does not remove the requirement to preserve the native CRDT information. [Hocuspocus persistence](https://tiptap.dev/docs/hocuspocus/guides/persistence)

A conceptual persistence layout to evaluate is:

| Representation | Role |
| --- | --- |
| Native CRDT snapshot | Reconstruct collaborative state, including merge identity |
| Incremental updates, optionally batched | Durability and synchronization since a snapshot |
| JSONB or relational projection | History/search/export and application queries where needed |

These are roles, not adopted table definitions. A provider may implement snapshot-only storage, updates with compaction, or another native format. If projections are asynchronous, their freshness and the canonical source of truth must be explicit. Avoid independently authoritative document and message copies with no reconciliation contract.

Updating one large JSONB value still locks its entire PostgreSQL row; CRDT convergence does not remove that storage behavior. Incremental persistence and compaction can avoid rewriting a growing snapshot for every keystroke, but introduce their own records and lifecycle. Moving from message rows to document snapshots/update records does not inherently eliminate storage growth or reduce total cost. [PostgreSQL JSON types](https://www.postgresql.org/docs/17/datatype-json.html)

#### What Can Be Automatic and What Remains Application Work

Yjs supports incremental, duplicate-tolerant merge updates and synchronization based on missing state. IndexedDB persistence retains the browser's local document across sessions; a separate network provider exchanges changes. These facilities address merge/sync mechanics rather than defining this application's server acceptance policy. [Yjs document updates](https://docs.yjs.dev/api/document-updates) and [offline support](https://docs.yjs.dev/getting-started/allowing-offline-editing).

An off-the-shelf sync server/provider can supply much of this machinery, but the Go backend does not gain it merely by adding JSONB. We must select a compatible CRDT implementation, persistence adapter, network protocol, authentication integration, and multi-backend propagation behavior. Some established stacks use a JavaScript collaboration server; that is a possible integration choice, not a requirement or selected replacement for Go. Existing Redis chat payloads are not already CRDT sync messages. Server acknowledgment must distinguish accepted/durable state from edits persisted only on one client.

#### Existing Message and Section Rules Still Apply

CRDT replica convergence means peers can agree on a merged state after exchanging the requisite changes. It does not automatically guarantee this application's valid nesting, contiguous sibling ranges, or source/destination boundary propagation rules. Two concurrent individually valid section transfers may require server arbitration or application-specific merge/repair semantics. No such policy has been selected.

The current `sequence_no` is a server-allocated chronological message sequence. A CRDT's deterministic list order or internal clock is not a replacement for that contract. Offline messages can appear locally as pending, but their final sequence needs assignment on server acceptance unless the user explicitly changes the ordering requirements.

Example: A ends at 20; an offline client composes a message targeted to A; another participant's ordinary message receives 21 before reconnect. The offline message would receive 22, so automatically placing it in A would violate the confirmed no-gap rule. The eventual behavior must be decided explicitly, such as rejecting section placement or requesting user reassignment. CRDT merging cannot choose that product policy. Offline creation of a section likewise raises how its provisional boundaries map to accepted message sequences.

Server-owned message metadata, actual send permission, who may edit existing content, and authorship must remain well-defined. Current active-sender enforcement must not disappear behind acceptance of arbitrary document mutations. Existing inactive-participant reads expose only through last-read sequence: a generic full-conversation sync room would not enforce that cutoff. Such readers could use the existing filtered history path or an explicitly authorized projection. A JSON rendering that hides later blocks after syncing them is insufficient to implement the selected read policy. This is compatibility with confirmed behavior, not a new restriction on delayed receipts.

#### Assessment and Next Comparison

This approach is more directly suited to live collaborative documents than frequently replacing one MinIO object. It is attractive if shared editing and offline merging are central product goals. It does not establish that one giant JSONB column is the right physical representation, nor that a full-document CRDT replacement is simpler than the current chat model.

Evaluate three architectural choices without assuming adoption:

1. A full CRDT conversation with native persistence and server-managed metadata/admission rules, plus query projections as needed.
2. Existing authoritative message/section operations with CRDTs restricted to content that genuinely needs collaborative text editing, such as drafts or selected message/heading bodies.
3. Existing authoritative operations and an IndexedDB pending-operation queue, if offline sends and section controls are sufficient without concurrent shared text editing.

Before choosing, clarify whether participants can edit one another's existing message content or just send their own messages and rearrange sections; define offline section-conflict behavior; and test large-history loading, update accumulation/compaction, filtered reads, and Go integration. No framework installation or backend change is authorized by this assessment alone.

### 9.6 Confirmed Working Inventory for Relational Schema Planning

The user has identified these four new tables as the basis for the next database-design discussion:

| Table | Required relationship/identity fields identified so far | Role |
| --- | --- | --- |
| `sections` | Section ID, conversation ID, header, start message sequence, end message sequence, parent section ID | Consecutive nested section structure, including empty sections |
| `section_references` | Reference ID, conversation ID, source message ID, referenced section ID | Links from messages to selected sections |
| `hashtags` | Hashtag ID, conversation ID, exact hashtag string | Conversation-local hashtag identity |
| `message_hashtags` | Association ID, conversation ID, message ID, hashtag ID | Searchable links from messages to hashtags |

The section header was not repeated in the user's latest shorthand inventory but remains an explicit prior requirement. Header text is non-unique; selected references resolve by ID.

Keep an `id` primary key on every new table. The user explicitly requires both `created_at` and `updated_at` on all four new tables. Exact timestamp defaults and update-maintenance mechanisms will be specified in the physical schema plan. `sections.created_at` preserves the chronology of section creation, including sections with no messages; edits must not replace that chronology with `updated_at`. Section creation authorship remains a recommended field to assess. The UUID generation policy for these new IDs is still open; existing UUIDv7 message generation is unchanged. This requirement does not authorize unrelated timestamp changes to existing tables.

No additional table is currently required solely for replies: `messages.reply_to_message_id` already represents the direct edge, and descendant threads can be queried from those edges. A suitable reply-target index should be considered. Likewise, a section-to-message membership join table is unnecessary for the chosen consecutive-range model. Receiving `section_id` as send intent does not by itself require storing a permanent `messages.section_id` column.

Including `conversation_id` on associations makes scope explicit. The schema plan must ensure that each association's message and target belong to that same conversation, and each parent section belongs to its child's conversation. Whether to use composite foreign keys, repository/service validation, or a combination remains to be settled; do not regard an independent conversation foreign key as sufficient to prove all linked entities share a conversation.

Before producing migration SQL, resolve:

- Distinguish already established constraints from remaining behavior choices: same-conversation links, exact conversation-local hashtag identity, stable section IDs with duplicate headers, and paired-null empty boundaries are not new open product questions. Choosing foreign keys/checks/indexes to express them is schema implementation work.
- Implement the confirmed association uniqueness rules: one row per message/hashtag pair and one per message/referenced-section pair, independent of occurrence count.
- Implement confirmed equal-boundary parent/child support and rejection of overlapping section creation outside valid ancestry. Express these constraints without breaking the established automatic propagation during explicit boundary transfers.
- First-message assignment for an eligible last empty section and last-section eligibility within nested hierarchies or tied creation timestamps.
- Exact constraints for paired nullable boundaries and parent containment during automatic multi-section adjustments.
- Target convention and escaping for the selected standard Markdown link syntax and the bulk-update contract for the confirmed opt-in rename behavior. Embedded target IDs already solve pairing repeated labels with their chosen targets, and association pair uniqueness is confirmed.
- Deletion behavior and minimal indexes implementing the confirmed uniqueness/search requirements.
- No reference-existence boolean is included; the user explicitly accepted omission. The physical schema proposal in section 9.7 is ready for review independently of remaining service/API details.

This is a table inventory and planning record, not authorization to apply schema changes or introduce additional infrastructure.

### 9.7 Proposed Physical Database Design

**Status: complete schema proposal for review, not an applied migration.** The behavior decisions below are confirmed; specific SQL names, indexes, and implementation defaults are engineering recommendations. This design retains PostgreSQL 17 and the existing separation of service policy, repository SQL, and module-local domain errors. No new storage system, extension, closure table, message-membership table, or thread table is required.

#### 9.7.1 Newly Confirmed Decisions

- Historical grouping follows message sequence placement: creating D over messages 1–9 after A (10–20) and C (21–30) places D before A. C remains eligible for message 31; the newest section creation timestamp does not override message chronology.
- Deleting a section removes its heading, promotes its direct children to its own parent (or the conversation root), and preserves all messages. Existing links to the deleted ID remain in message content and render as unavailable.
- Omit `messages.has_section_references`. Association existence is sufficient.

These clarify the earlier wording about creation chronology: `created_at` remains the record of section creation and the chronology available for empty headings; it does not reorder nonempty historical ranges. Exact merging of empty headings into a rendered outline is a frontend/service rule, not an excuse to order nonempty sections solely by creation time.

#### 9.7.2 Common Columns and Defaults

- Every new table has `id UUID PRIMARY KEY`, `created_at TIMESTAMPTZ NOT NULL`, and `updated_at TIMESTAMPTZ NOT NULL`.
- **Recommended:** generate all four new table IDs in Go using the existing `uuid.NewV7()`, as for message IDs. No database UUIDv4 default, PostgreSQL upgrade, or version-check constraint is needed. Existing unrelated ID generators remain unchanged.
- **Recommended:** timestamp defaults use `clock_timestamp()`, recording insertion time rather than a transaction's possibly earlier start time after waiting for a conversation lock. Repositories explicitly set `updated_at = clock_timestamp()` on actual updates; `created_at` remains unchanged. No timestamp trigger is introduced.
- Equal creation timestamps can be displayed with `id` as a deterministic tie-breaker, without pretending that this creates message-sequence chronology.
- `sections.created_by` records heading authorship. Association authorship is already derivable from the source message; no additional author column is needed there.
- Headers are nonblank text and may repeat. Hashtag names are exact text with deterministic `"C"` collation, avoiding case-folded identity. **Recommended literal convention:** store the full literal, such as `#backend`, without trimming/lowercasing; the parser contract must consistently produce it. The schema does not enforce a hashtag grammar or arbitrary length limit.
- The four-table timestamps apply only to these new tables; unrelated old tables are not being redesigned.

#### 9.7.3 Candidate DDL

This SQL is a design artifact. It assumes the current initial migration baseline and is not a new migration file.

```sql
ALTER TABLE messages
    ADD CONSTRAINT messages_conversation_id_id_key UNIQUE (conversation_id, id);

ALTER TABLE messages
    DROP CONSTRAINT messages_reply_to_message_id_fkey,
    ADD CONSTRAINT messages_reply_to_conversation_message_fkey
        FOREIGN KEY (conversation_id, reply_to_message_id)
        REFERENCES messages (conversation_id, id);

CREATE INDEX idx_messages_conversation_reply_sequence
    ON messages (conversation_id, reply_to_message_id, sequence_no)
    WHERE reply_to_message_id IS NOT NULL;

CREATE TABLE sections (
    id UUID PRIMARY KEY,
    conversation_id UUID NOT NULL REFERENCES conversations(id),
    header TEXT NOT NULL,
    start_sequence_no BIGINT,
    end_sequence_no BIGINT,
    parent_section_id UUID,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT sections_conversation_id_id_key UNIQUE (conversation_id, id),
    CONSTRAINT sections_parent_conversation_fkey
        FOREIGN KEY (conversation_id, parent_section_id)
        REFERENCES sections (conversation_id, id),
    CONSTRAINT sections_start_message_fkey
        FOREIGN KEY (conversation_id, start_sequence_no)
        REFERENCES messages (conversation_id, sequence_no),
    CONSTRAINT sections_end_message_fkey
        FOREIGN KEY (conversation_id, end_sequence_no)
        REFERENCES messages (conversation_id, sequence_no),
    CONSTRAINT sections_not_own_parent CHECK (
        parent_section_id IS NULL OR parent_section_id <> id
    ),
    CONSTRAINT sections_valid_boundaries CHECK (
        (start_sequence_no IS NULL AND end_sequence_no IS NULL)
        OR (
            start_sequence_no IS NOT NULL
            AND end_sequence_no IS NOT NULL
            AND start_sequence_no >= 1
            AND end_sequence_no >= start_sequence_no
        )
    ),
    CONSTRAINT sections_nonempty_header CHECK (length(btrim(header)) > 0)
);

CREATE INDEX idx_sections_conversation_parent_created
    ON sections (conversation_id, parent_section_id, created_at, id);

CREATE INDEX idx_sections_conversation_boundaries
    ON sections (conversation_id, start_sequence_no, end_sequence_no)
    WHERE start_sequence_no IS NOT NULL;

CREATE TABLE section_references (
    id UUID PRIMARY KEY,
    conversation_id UUID NOT NULL REFERENCES conversations(id),
    message_id UUID NOT NULL,
    referenced_section_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT section_references_message_fkey
        FOREIGN KEY (conversation_id, message_id)
        REFERENCES messages (conversation_id, id) ON DELETE CASCADE,
    CONSTRAINT section_references_section_fkey
        FOREIGN KEY (conversation_id, referenced_section_id)
        REFERENCES sections (conversation_id, id) ON DELETE CASCADE,
    CONSTRAINT section_references_message_target_key
        UNIQUE (message_id, referenced_section_id)
);

CREATE INDEX idx_section_references_conversation_target_message
    ON section_references (conversation_id, referenced_section_id, message_id);

CREATE TABLE hashtags (
    id UUID PRIMARY KEY,
    conversation_id UUID NOT NULL REFERENCES conversations(id),
    name TEXT COLLATE "C" NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT hashtags_conversation_id_id_key UNIQUE (conversation_id, id),
    CONSTRAINT hashtags_conversation_name_key UNIQUE (conversation_id, name),
    CONSTRAINT hashtags_nonempty_name CHECK (length(name) > 0)
);

CREATE TABLE message_hashtags (
    id UUID PRIMARY KEY,
    conversation_id UUID NOT NULL REFERENCES conversations(id),
    message_id UUID NOT NULL,
    hashtag_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT message_hashtags_message_fkey
        FOREIGN KEY (conversation_id, message_id)
        REFERENCES messages (conversation_id, id) ON DELETE CASCADE,
    CONSTRAINT message_hashtags_hashtag_fkey
        FOREIGN KEY (conversation_id, hashtag_id)
        REFERENCES hashtags (conversation_id, id),
    CONSTRAINT message_hashtags_message_target_key UNIQUE (message_id, hashtag_id)
);

CREATE INDEX idx_message_hashtags_conversation_tag_message
    ON message_hashtags (conversation_id, hashtag_id, message_id);
```

The migration's existing `UNIQUE (conversation_id, sequence_no)` supports boundary foreign keys. The additional `UNIQUE (conversation_id, id)` keys support composite foreign keys that enforce same-conversation relationships in PostgreSQL. They do add indexes; they are deliberate integrity costs, not additional message rows. Do not add duplicate indexes for primary keys or unique constraints. PostgreSQL requires a referenced foreign-key column set to have a suitable unique key. [PostgreSQL 17 constraints](https://www.postgresql.org/docs/17/ddl-constraints.html)

The paired-boundary check explicitly tests both fields for non-null before comparing them. A short check containing only comparisons can accidentally accept a one-null boundary because SQL CHECK also accepts a null result.

#### 9.7.4 What PostgreSQL Enforces

- Unique identities, required columns, nonblank section headers, and nonempty hashtag names.
- Both boundaries null, or both non-null with positive start and inclusive end greater than or equal to start.
- Existing start/end messages in the section's conversation. Null endpoints skip the corresponding foreign-key match, as required for empty sections.
- Parent section, referenced section, referenced hashtag, source message, and reply target belong to the same conversation as their owning row.
- A section cannot directly parent itself; longer ancestry cycles require service validation.
- Exact hashtag identity unique within the conversation.
- One reference association per message/target-section pair, and one hashtag association per message/tag pair.

There is no unique section-header constraint and no constraint preventing a child from having its parent's exact boundaries.

#### 9.7.5 What Services Validate Transactionally

Cross-row hierarchy policy stays in services, using repository methods to fetch/check persisted state. PostgreSQL CHECK constraints cannot express general parent containment, ancestry cycles, or arbitrary comparisons against other rows. [PostgreSQL 17 CHECK restrictions](https://www.postgresql.org/docs/17/ddl-constraints.html)

Services must validate:

- Acyclic ancestry and valid containment for nonempty descendants; an empty parent cannot remain empty while it contains a nonempty descendant.
- No overlap between unrelated/sibling nonempty sections; ancestor/descendant containment, including equal boundaries, is permitted.
- New historical-group creation must reject conflicting overlap instead of silently changing existing sections. An intended new ancestor/descendant relationship must be explicit.
- Explicit tail-send eligibility, consecutive extension, and the fact that ordinary sends never extend sections.
- Historical range order rather than creation order when determining the last nonempty section. Empty-heading eligibility must still respect the agreed chronology rule; the exact nested/tie/empty first-assignment contract belongs to the remaining service/API design.
- Boundary removals propagate inner-to-outer; insertions propagate outer-to-inner. Validate the final affected tree without dropping unrelated sibling content.
- Reference/tag parsing and derivation from canonical message content. Duplicate textual occurrences do not produce duplicate associations.
- Exact immediate reply targets and chronological reply admission; do not substitute the oldest ancestor ID.

**Recommended locking protocol:** use one transaction, check participant state using the existing repository lookup, then lock the owning conversation row before reading/modifying its structure. This matches the current sender's participant-check-then-conversation-counter sequence. Serialize structural mutations and relevant message mutations through that same conversation row. Compute intended changes in the service, execute SQL through repositories, commit once, then publish events. This makes a concurrent overlap check meaningful instead of allowing two transactions to validate stale structures independently.

Do not introduce a global range-exclusion constraint: rejecting every overlapping range would also reject required nesting and equal-boundary parent/child sections. A sibling-only exclusion design would still need cycle/containment checks and additional treatment of null parents. The initial proposal uses ordinary constraints plus conversation-serialized service checks, preserving the requested architecture without a new GiST extension.

Direct administrative SQL can bypass service-owned hierarchy rules. This is a deliberate boundary of this proposal, not a claim that row CHECK constraints enforce the whole tree. Review a trigger-based design only if database-enforced cross-row hierarchy becomes an explicit requirement.

#### 9.7.6 Deletion and Content Lifecycle

| Operation | Proposed database/application outcome |
| --- | --- |
| Delete a section | In one conversation-locked transaction, reparent its direct children to its former parent/root, validate the final hierarchy, then delete the section |
| Links to that deleted section | Content stays byte-for-byte unchanged; target lookup becomes unavailable; target association rows cascade away |
| Links to promoted children | Their IDs remain unchanged, so they remain resolvable; stored path labels are not automatically rewritten by section deletion |
| Messages formerly covered by deleted heading | Remain at their original sequences; an ancestor still covers its range where applicable, otherwise they are ungrouped except for surviving children |
| Hard-delete a source message | Its reference/tag associations cascade away; existing reply and section-boundary foreign keys can still block deletion until deliberately resolved |
| Soft-delete a message | Existing message row/sequence remains, preserving range and reply identity; payload visibility and searchable-association cleanup require the later deletion service contract |
| Delete a hashtag still referenced by messages | Rejected by its foreign key; remove associations deliberately before dictionary cleanup |
| Remove a hashtag's final association | Dictionary row can remain; no automatic garbage-collection trigger or extra table |
| Delete a conversation/user | Existing application deletion policy and foreign-key restrictions remain; this proposal does not implement the existing stub routes |

Hard-deleting a section does not rewrite Markdown links to remove their UUIDs. Their stored labels are sufficient for the frontend to show an unavailable link. Reference search by a deleted section's current name is no longer available because that section no longer exists; no historical-name/tombstone table is added.

Renaming with opt-in updates must update persisted message labels across the intended author's history, matching by target ID. Target associations stay the same when only labels change. A normal message edit changing targets/tags must reconcile both association sets within its transaction. Keep unchanged association rows where possible so their identity/creation time is stable.

#### 9.7.7 Index and Query Plan

| Access pattern | Supporting index/constraint |
| --- | --- |
| Existing history pagination | Existing unique conversation/sequence key |
| Direct replies for recursive descendant traversal | New partial conversation/reply-target/sequence index |
| All sections in a conversation / children of a section | Conversation/parent/creation/ID index |
| Sections containing/intersecting a sequence range | Partial conversation/start/end index; final ancestor filtering remains explicit |
| Resolve section by ID | Primary key, scoped by conversation in repository query |
| Render references for a page, if needed | Message/target unique key |
| Find messages referencing a section | Conversation/target-section/message index, then sequence-paginated join to messages |
| Find exact hashtag in a conversation | Conversation/name unique key |
| Find messages with a hashtag | Conversation/hashtag/message index, then sequence-paginated join to messages |

Fetch message pages before aggregating their associations, or use EXISTS/join patterns that return each message once. Repeated link/tag occurrences must not multiply results. Return relevant IDs/labels in API arrays if needed without duplicating message rows.

Search by header does not require another table. Initial scoped text matching can use the conversation-filtered section set; choose substring/prefix/full-text behavior during API planning before adding pg_trgm or full-text indexes. Likewise, keyword message search need not introduce Elasticsearch or a search table in this schema change.

Do not add a cached header/path column to reference rows, a cached count, a threads table, a section-membership join table, or the omitted boolean merely to avoid ordinary indexed queries.

#### 9.7.8 Changes to Existing Tables and Migration Shape

The only proposed existing-table schema changes are:

1. Add the scoped message identity unique key needed by the new composite foreign keys.
2. Replace the old reply-target foreign key with a same-conversation composite foreign key, preserving the exact reply target and existing non-cascading delete behavior.
3. Add the partial reply lookup index.

No message content column conversion, per-message section column, sequence renumbering, access-policy change, or read-receipt restriction is required.

For a later authorized implementation, use one new Goose migration: alter messages, create sections, section_references, hashtags, and message_hashtags, then supporting indexes. Down drops associations before target dictionaries/sections, drops the new reply index/foreign key, restores the original reply foreign key, and removes the new scoped identity key after its dependents are gone. Keep `schema.sql` synchronized for these changes only; its existing unrelated differences are not authorization for a wholesale schema rewrite. Inspect any existing cross-conversation reply data before replacing the reply constraint rather than silently repairing it.

The application is not in production and a fresh database remains possible, but this design does not authorize a reset or rewriting existing migrations.

#### 9.7.9 Validation Plan

Validate candidate SQL in a disposable schema/transaction before implementation. At migration time, verify:

- Empty paired-null boundaries accepted; one-null, reversed, nonpositive, nonexistent, and cross-conversation endpoints rejected.
- Same-conversation parent/reference/tag/reply links accepted; cross-conversation links rejected.
- Duplicate headers and exact parent/child ranges accepted.
- Exact duplicate tag literals within one conversation rejected, while case variants and another conversation's identical literal are accepted.
- Duplicate message/section and message/tag associations rejected.
- Section deletion with explicit child promotion preserves all messages and child links while cascading only links' association rows for the deleted target; original payload labels/IDs remain.
- Services reject cycles, unrelated overlaps, nonconsecutive placements, and invalid parent/child states, including concurrent conflicting mutations.
- Existing UUIDv7 message creation, sequence pagination, active/inactive history policy, and delayed receipts remain unchanged.

Only the first group of static database rules is enforceable by the DDL itself. Service behavior is validated when those operations are implemented; the schema's existence alone is not proof they work.


#### 9.7.10 DBML Diagram

Copy the following block into a DBML-compatible diagram tool such as dbdiagram.io. It includes the four proposed tables and the existing users, conversations, and messages tables needed to show their relationships; unrelated authentication/device tables are omitted. Composite references preserve the proposed same-conversation foreign keys.

The PostgreSQL DDL in section 9.7.3 remains authoritative. Partial-index predicates and the hashtag column's collation are documented as DBML notes rather than executable settings, so exported SQL is not a replacement migration. Hierarchy policy and child promotion remain service responsibilities. Syntax follows the [official DBML reference](https://dbml.dbdiagram.io/docs/).

```dbml
Project markdown_chat {
  database_type: 'PostgreSQL'
  Note: 'Proposed PostgreSQL 17 schema. Includes four new tables and the three existing tables they depend on. SQL in section 9.7.3 is authoritative for partial indexes and collation.'
}

Table users {
  id uuid [pk, not null, default: `gen_random_uuid()`]
  user_name text [not null]
  email text [unique]
  password text [not null]
  created_at timestamptz [not null, default: `now()`]
}

Table conversations {
  id uuid [pk, not null, default: `gen_random_uuid()`]
  created_at timestamptz [not null, default: `now()`]
  type text [not null, default: 'direct']
  title text
  description text
  avatar_url text
  created_by uuid [not null]
  next_sequence_no bigint [not null, default: 1]

  checks {
    `type IN ('direct', 'group')`
  }
}

Table messages {
  id uuid [pk, not null, note: 'UUIDv7 generated in Go']
  created_at timestamptz [not null, default: `now()`]
  sender_id uuid [not null]
  conversation_id uuid [not null]
  content text [not null, note: 'Canonical Markdown with self-contained section links']
  deleted_at timestamptz
  updated_at timestamptz
  reply_to_message_id uuid [note: 'Exact immediate reply target; same conversation']
  sequence_no bigint [not null, default: 1]
  client_message_id uuid

  indexes {
    (conversation_id, id) [unique, name: 'messages_conversation_id_id_key']
    (conversation_id, sequence_no) [unique]
    (sender_id, client_message_id) [unique]
    (conversation_id, sequence_no) [name: 'idx_messages_conversation_sequence', note: 'Existing redundant index retained by this proposal']
    sender_id [name: 'idx_messages_sender']
    (conversation_id, reply_to_message_id, sequence_no) [name: 'idx_messages_conversation_reply_sequence', note: 'Partial in PostgreSQL: WHERE reply_to_message_id IS NOT NULL']
  }
}

Table sections {
  id uuid [pk, not null, note: 'UUIDv7 generated in Go']
  conversation_id uuid [not null]
  header text [not null, note: 'Nonblank; duplicate headers allowed']
  start_sequence_no bigint [note: 'Inclusive; NULL together with end for empty sections']
  end_sequence_no bigint [note: 'Inclusive; NULL together with start for empty sections']
  parent_section_id uuid [note: 'NULL for a top-level section']
  created_by uuid [not null]
  created_at timestamptz [not null, default: `clock_timestamp()`]
  updated_at timestamptz [not null, default: `clock_timestamp()`]

  indexes {
    (conversation_id, id) [unique, name: 'sections_conversation_id_id_key']
    (conversation_id, parent_section_id, created_at, id) [name: 'idx_sections_conversation_parent_created']
    (conversation_id, start_sequence_no, end_sequence_no) [name: 'idx_sections_conversation_boundaries', note: 'Partial in PostgreSQL: WHERE start_sequence_no IS NOT NULL']
  }

  checks {
    `parent_section_id IS NULL OR parent_section_id <> id` [name: 'sections_not_own_parent']
    `(start_sequence_no IS NULL AND end_sequence_no IS NULL) OR (start_sequence_no IS NOT NULL AND end_sequence_no IS NOT NULL AND start_sequence_no >= 1 AND end_sequence_no >= start_sequence_no)` [name: 'sections_valid_boundaries']
    `length(btrim(header)) > 0` [name: 'sections_nonempty_header']
  }

  Note: 'Service-owned rules: acyclic hierarchy, containment, no unrelated overlap, consecutive changes, and deletion with child promotion. Identical parent/child ranges are allowed.'
}

Table section_references {
  id uuid [pk, not null, note: 'UUIDv7 generated in Go']
  conversation_id uuid [not null]
  message_id uuid [not null]
  referenced_section_id uuid [not null]
  created_at timestamptz [not null, default: `clock_timestamp()`]
  updated_at timestamptz [not null, default: `clock_timestamp()`]

  indexes {
    (message_id, referenced_section_id) [unique, name: 'section_references_message_target_key']
    (conversation_id, referenced_section_id, message_id) [name: 'idx_section_references_conversation_target_message']
  }

  Note: 'Derived from message content. Repeated links to the same section produce one association. Deleting the target removes associations but preserves original links in message text.'
}

Table hashtags {
  id uuid [pk, not null, note: 'UUIDv7 generated in Go']
  conversation_id uuid [not null]
  name text [not null, note: 'PostgreSQL COLLATE "C"; exact case and spelling; proposed full literal such as #backend']
  created_at timestamptz [not null, default: `clock_timestamp()`]
  updated_at timestamptz [not null, default: `clock_timestamp()`]

  indexes {
    (conversation_id, id) [unique, name: 'hashtags_conversation_id_id_key']
    (conversation_id, name) [unique, name: 'hashtags_conversation_name_key']
  }

  checks {
    `length(name) > 0` [name: 'hashtags_nonempty_name']
  }
}

Table message_hashtags {
  id uuid [pk, not null, note: 'UUIDv7 generated in Go']
  conversation_id uuid [not null]
  message_id uuid [not null]
  hashtag_id uuid [not null]
  created_at timestamptz [not null, default: `clock_timestamp()`]
  updated_at timestamptz [not null, default: `clock_timestamp()`]

  indexes {
    (message_id, hashtag_id) [unique, name: 'message_hashtags_message_target_key']
    (conversation_id, hashtag_id, message_id) [name: 'idx_message_hashtags_conversation_tag_message']
  }

  Note: 'One association per message/tag pair, independent of textual occurrence count. Hashtags are searched explicitly and have no click action.'
}

Ref: conversations.created_by > users.id [delete: no action]
Ref: messages.sender_id > users.id [delete: no action]
Ref: messages.conversation_id > conversations.id [delete: no action]
Ref messages_reply_to_conversation_message_fkey: messages.(conversation_id, reply_to_message_id) > messages.(conversation_id, id) [delete: no action]

Ref: sections.conversation_id > conversations.id [delete: no action]
Ref: sections.created_by > users.id [delete: no action]
Ref sections_parent_conversation_fkey: sections.(conversation_id, parent_section_id) > sections.(conversation_id, id) [delete: no action]
Ref sections_start_message_fkey: sections.(conversation_id, start_sequence_no) > messages.(conversation_id, sequence_no) [delete: no action]
Ref sections_end_message_fkey: sections.(conversation_id, end_sequence_no) > messages.(conversation_id, sequence_no) [delete: no action]

Ref: section_references.conversation_id > conversations.id [delete: no action]
Ref section_references_message_fkey: section_references.(conversation_id, message_id) > messages.(conversation_id, id) [delete: cascade]
Ref section_references_section_fkey: section_references.(conversation_id, referenced_section_id) > sections.(conversation_id, id) [delete: cascade]

Ref: hashtags.conversation_id > conversations.id [delete: no action]
Ref: message_hashtags.conversation_id > conversations.id [delete: no action]
Ref message_hashtags_message_fkey: message_hashtags.(conversation_id, message_id) > messages.(conversation_id, id) [delete: cascade]
Ref message_hashtags_hashtag_fkey: message_hashtags.(conversation_id, hashtag_id) > hashtags.(conversation_id, id) [delete: no action]
```


## 10. Unresolved Decisions and Next Discussion

The initial behavior questions and the narrower follow-ups are answered: consecutive ranges, selected reference IDs, independent timeline replies, permitted empty sections, directional automatic boundary propagation, explicit section targeting, and flat views of all reply descendants. Resolve the following before producing an implementation-ready plan:

- Physical constraints for paired nullable sequence boundaries; empty sections use both null and retain creation chronology through `created_at`.
- First-message assignment for the last eligible empty section, nested/tied last-section eligibility, and how a section is explicitly ended without starting another section.
- Exact affected-path boundary algorithms, shared ancestors during transfer, and preserving other content while propagating adjustments.
- Flat descendant-thread ordering, pagination, deleted-message handling, and query/index strategy.
- Concurrent boundary-edit conflicts, nesting limits, and section deletion/content preservation. Overlapping creation is already confirmed to be rejected; equal-boundary parent/child sections are allowed.
- Who can create, rename, resize, move, or delete sections in a conversation.
- Link target encoding/escaping, bulk-label-update scope/contracts for opt-in renames, unknown references, target deletion, and synchronization of derived references on message edits. Message/target pair uniqueness is confirmed.
- Review the physical schema proposal in section 9.7; its engineering defaults do not implement the remaining service/API behaviors.
- Hashtag syntax and association lifecycle.
- Search semantics: exact versus partial header matching, content matching, descendant inclusion, reply inclusion, and pagination.
- Review/approve the proposed physical SQL and indexes in section 9.7, then finalize service mutation/concurrency contracts before implementing behavior.
- APIs, socket commands/events, multi-device updates, and reconnect recovery.
- Whether actual Markdown export is required; the user requested representability and Markdown mode, but an export endpoint and serialization format have not been specified.
- Migration strategy for this feature, compatibility expectations, and focused acceptance tests.

## 11. Change History

### 2026-10-23 — Added DBML Diagram

- Added a copyable DBML block for all four proposed tables and their three existing dependencies, including indexes, checks, same-conversation composite references, and delete actions.
- Validated DBML parsing/export with the DBML CLI and checked that all four new-table column sets match the candidate SQL.
- Marked partial-index predicates and PostgreSQL collation as diagram notes, retaining candidate SQL as the migration authority. No schema or backend changes were made.

### 2026-10-23 — Complete Physical Schema Proposal and Database Validation

- Confirmed historical range placement, heading deletion with direct-child promotion/messages preserved, and omission of the reference-existence boolean.
- Added section 9.7 with candidate DDL for the four tables, shared defaults, composite same-conversation foreign keys, indexes, deletion behavior, service-owned hierarchy validation, and a later Goose migration plan.
- Proposed UUIDv7 generation in Go for the new IDs, exact-text hashtag storage, explicit repository timestamp updates, and one conversation-serialized transaction for structural mutations. These engineering details are proposals, not claims of additional user decisions.
- Executed the candidate DDL and static checks in an isolated PostgreSQL 17 transaction; all checks passed and the transaction rolled back. No application schema or migration files changed.
- Updated frontend flows for historical positioning and deletion with unavailable reference links. Remaining route, permission, and complex mutation details are kept separate from this physical database design.

### 2026-10-23 — Association Uniqueness, Hashtag Interaction, and Section Overlap Confirmed

- Confirmed one `message_hashtags` row per message/hashtag pair regardless of repeated literal occurrences, and one `section_references` row per message/target-section pair regardless of repeated links.
- Confirmed hashtags have no click action; tagged messages are retrieved by explicit exact-string search within the conversation. Section links retain their navigation behavior.
- Confirmed identical parent/child sequence boundaries are allowed. Reject overlapping section creation outside valid ancestor/descendant containment rather than changing existing sections implicitly.
- Updated both backend and frontend context summaries, behavior sections, and open-decision lists. No schema or application code changed.

### 2026-10-23 — Standard Links, Opt-In Historical Label Updates, and Frontend Companion

- Confirmed standard `[display label](target ID)` serialization, superseding the original reversed-delimiter example.
- Confirmed the frontend rename prompt: yes updates the user's older message content; no preserves old reference labels. Target IDs and message sequences remain unchanged. Recorded own-author scope as the working interpretation without authorizing edits of others' messages.
- Recorded that backend bulk historical updates, edit/rename routes, partial-failure contracts, and update events remain to be designed.
- Created the separate maintained frontend design document with all current API contracts, full user flows, Markdown behavior, open integration dependencies, and a reusable context summary.
- Updated both design records without changing backend behavior or creating a frontend implementation.

### 2026-10-23 — Proposed Self-Contained Section Links in Message Content

- Recorded the user's selected-label-plus-ID inline representation and derivation of `section_references` from stored content.
- Recorded the lookup role of association rows and direct rendering/navigation from embedded IDs, eliminating the need to pair inline occurrences with separately fetched reference rows.
- Distinguished the user's `(label)[id]` example from standard Markdown `[label](target)` syntax; the assistant recommends the latter, but exact serialization remains unselected.
- Recorded canonical content versus derived association responsibilities and retained rename/search/uniqueness details as open decisions. No schema or backend changes were made.

### 2026-10-23 — Empty-Section Representation and Timestamps Confirmed

- Confirmed nullable `sections.start_sequence_no` and `sections.end_sequence_no`, with both null for an empty section, including after its last message is transferred away.
- Confirmed `created_at` as the record of section creation chronology and both `created_at` and `updated_at` on all four new tables.
- Recorded A = 10–20, B = empty, C = 21–30 in creation order: a newly sent message 31 cannot target B because it is not the last section. Empty state does not bypass append eligibility.
- Updated the context summary and removed empty-section representation from unresolved behavior. Kept exact constraints, timestamp handling, and nested eligibility details for the physical database plan. No schema or backend changes were made.

### 2026-10-23 — Returned to Four-Table Relational Schema Planning

- Recorded the explicit decision to scrap the file-based conversation approach and return to the existing relational design.
- Confirmed the working inventory: `sections`, `section_references`, `hashtags`, and `message_hashtags`, including the user's explicit conversation IDs on both association tables.
- Noted the previously required section header, standard identity/creation fields, and remaining empty-section and inline-reference representation questions.
- Recorded that existing reply relationships and range-derived membership do not inherently require new thread or membership tables.
- Updated the reusable context summary. No database migration or backend change was made.

### 2026-10-23 — File-First Direction Set Aside

- Recorded the user's clarified motivation: substantially fewer database rows and tables, rather than a requirement for a physical conversation file.
- Recorded their assessment that the file approach has more challenges than benefits and set aside file-first replacement for now.
- Retained the feasibility analyses as context without adopting CRDTs, MinIO snapshots, Elasticsearch, or a new storage architecture. No schema or backend code changed.

### 2026-10-23 — Assessed CRDT Collaborative-Document Direction

- Recorded the follow-up idea of browser IndexedDB, CRDT synchronization, and PostgreSQL document persistence without treating it as an adoption decision.
- Distinguished JSONB projections from native CRDT merge state, including Yjs binary-state requirements and potential PostgreSQL `BYTEA` persistence.
- Assessed incremental updates/snapshots, same-row JSONB contention, framework/provider integration, and the continued need for sequence assignment, section invariants, and the confirmed per-operation access policy.
- Recorded the concrete offline section-placement conflict and alternatives ranging from full CRDT documents to selective collaborative text or a pending-send queue.
- Updated the reusable context summary. No code, database, dependency, or infrastructure changes were made.

### 2026-10-23 — Assessed File-Backed Conversation Idea Without Adopting It

- Recorded the user's exploratory idea of one MinIO conversation object and Elasticsearch keyword search, explicitly preserving its non-proposal status.
- Assessed document representation benefits, whole-object replacement costs, deployment-specific append capability, concurrent writers, cross-store commit behavior, pagination, threads, and inactive-participant visibility.
- Separated adopting a search index from replacing primary message storage. Recorded that cross-conversation search exploration does not change conversation-scoped section and hashtag identity requirements.
- Recorded the assistant's current preference for structured canonical storage plus generated Markdown snapshots as a recommendation only.
- Updated the reusable context summary. No backend, schema, dependency, or infrastructure changes were made.

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
