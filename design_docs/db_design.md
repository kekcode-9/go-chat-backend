Every table, has to have a `id` column of type `uuid`.
Every table, has to have a `created_at` column with default timestamp.
`id` has to be the primary key of every table even if it has any other column or composite columns specified as unique.
# `refresh_sessions` table
```
id: uuid
user_id: uuid
device_id: uuid
refresh_token_hash: string
expires_at: timestamp
revoked: boolean
created_at: timestamp
last_used_at: timestamp

user_id is Foreign key(users.id)
device_id is Foreign key(devices.id)
```
# `users` table
```
id: uuid,
user_name: string,
email: string || null,
password: hash,
created_at: timestamp

UNIQUE(email)
UNIQUE(user_name)
```

# `conversations` table
```
id: uuid,
created_at: timestamp,
type: "direct" || "group", // default "direct",
title: string || null,
description: string || null,
avatar_url: string || null,
created_by: uuid,
next_sequence_no: BIGINT default 1

created_by is Foreign key(users.id)
```
Before creating a direct conversation, search for an existing direct conversation with exactly those two participants. If one exists, return it instead of creating a new one.
# `conversation_participants` table
```
id: uuid,
created_at: timestamp,
role: "member" || "admin",
conversation_id: uuid,
user_id: uuid,
left_at: timestamp || null,
is_muted: boolean, // default False
is_archived: boolean, // default False
is_pinned: boolean, // default False
last_read_messsage_id: uuid || null,
last_read_mssg_seq: BIGINT || null

conversation_id is Foreign key(conversations.id)
user_id is Foreign key(users_id)
last_read_message_id is Foreign key(messages.id)
(conversation_id, user_id) should be unique

INDEX(user_id) // Listing all convos of a user
```

When updating `last_read_message_id`, `last_read_mssg_seq` also has to be updated and vice versa.
# `messages` table
I would **never** make `messages.id` a BIGSERIAL.

Reasons:
- UUIDs are excellent public identifiers.
- Easier sharding later.
- Easier merging databases.
- Harder to enumerate.
- Stable API IDs.

```
id: uuid,
created_at: timestamp,
sender_id: uuid,
conversation_id: uuid,
content: string,
deleted_at: timestamp || null,
updated_at: timestamp || null,
reply_to_message_id: uuid || null,
sequence_no: BIGINT default 1
client_message_id: uuid || null

conversation_id is Foreign key(conversations.id)
sender_id is Foreign_key(users.id)
reply_to_message_id is Foreign key(messages.id)

UNIQUE(conversation_id, sequence_no)

if updated_at !== created_at, the message has been edited

INDEX(conversation_id, sequence_no) // will need when loading entire convo
INDEX(sender_id)
```
(conversation_id, sequence_no) should be unique but sequence_no need not be unique across conversations

## Idempotency
`client_message_id` is sent from the client device itself and is used to ensure that in an event like a network fail if the same message gets retried, it is done so with the same `client_message_id` and server ensures uniqueness of `client_message_id` so it doesn't end up adding the same message twice.
## Different message types in future
Later, don't mutate the `content` column into JSON.
Instead, introduce a new column such as
```
message_typeTEXTIMAGEVIDEODOCUMENT...
```
and keep `content` as the textual payload for text messages while media-specific information lives in separate tables.

# `devices` table
```
id: uuid,
user_id: uuid,
device_name: string,
device_type: string,
public_key: Text, // later for uuid
last_seen_at: timestamp,
created_at: timestamp,
revoked_at: timestamp || null, // sign out case

INDEX(user_id)
```
# `blocked_users` table
```
id: uuid,
created_at: timestamp,
blocker_id: uuid,
blocked_id: uuid

blocker_id is Foreign key(users.id)
blocked_id is Foreign key(users.id)
UNIQUE(blocker_id, blocked_id)
```
# Rule for syncing message in each device
The app loaded in each device maintains a storage of itself where it tracks important states especially related to that device. Among these states there should be one:
```
covo_last_mssg = {
	[conversation_id: uuid]: [message_seq_no: BIGINT]
}
```
The `conversation_id` has to come from the server's database and so does the `message_seq_no`.
This state keeps track of the last read message in this particular device for each of the conversations.
When the device comes online, send this state to the server in a HTTP request body and for each of the `conversation_id`, `message_seq_no` pair, the server will query the `messages` table to find all messages with the same `conversation_id` whose `sequence_no` is larger than the `message_seq_no` received from the device and it will send over all those messages for each conversation in the request.
# Rule for updating `conversation_participants.last_read_mssg_seq`
This update attempt will always be made with a `message_id` in hand.
Query the `messages` table to retrieve the `sequence_no` of `message_id` to update with. If this `sequence_no` is larger than the `last_read_mssg_seq` of the corresponding `conversation_participants` row, only then should you update the `last_read_mssg_seq`, not otherwise.
When updating `last_read_mssg_seq` also update `last_read_message_id` and vice versa.
# Rule for adding a new entry to `messages`
Multiple POST requests might come at the exact same time. Writing to the `messages` table should be atomic. When inserting a new entry, its `sequence_no` should be taken from `conversations.next_sequence_no` which should then as part of the same atomic transaction be incremented by 1.

# Read receipts
```sql
SELECT
    user_id,
    last_read_sequence_no >= 156 AS has_read
FROM conversation_participants
WHERE conversation_id = ?
```
# Finding people
Search with `user_name` or `user_email`
No need to show contacts list

---
# Future tables
I think you've reached a really good milestone. If we freeze your schema today, **you can already build a working WhatsApp clone (Tier 1)**.

The most important thing is that I don't see any architectural dead ends. Every remaining feature can be added by introducing new tables instead of modifying existing ones.

There are only two schema changes I'd make before discussing new tables.

---

# First: Don't make `last_read_mssg_seq` a foreign key

This line

```text
last_read_mssg_seq is Foreign key(messages.sequence_no)
```

is actually incorrect.

Why?

Because

```text
messages.sequence_no
```

is **not globally unique**.

It's only unique together with

```text
(conversation_id, sequence_no)
```

A foreign key must reference either

- a primary key
    
- or a UNIQUE column
    

But

```text
sequence_no
```

alone is neither.

In reality, `last_read_mssg_seq` should just be

```text
BIGINT
```

with **no foreign key**.

The consistency is maintained by your application logic.

---

# Second: `public_key`

You wrote

```text
public_key: uuid
```

This should not be UUID.

Eventually it'll be

```text
TEXT

or

BYTEA
```

because it stores the cryptographic public key.

That's a future problem, but worth noting.

---

# Now, what tables are still missing?

I would split them into **Tier 2** and **Tier 3**.

---

## Tier 2

### 1. message_receipts

Notice

Your

```text
last_read_sequence_no
```

only tells us

> has user read this?

It does **not** tell us

- sent
    
- delivered
    
- failed
    

For WhatsApp-like ticks you'll eventually need

```text
message_receipts

----------------

id

created_at

message_id

user_id

status

updated_at
```

Status

```text
SENT

DELIVERED

READ
```

Notice

No

```text
conversation_id
```

needed.

It comes through

```text
message
```

---

### 2. message_reactions

```text
id

created_at

message_id

user_id

reaction
```

Unique

```text
(message_id,user_id)
```

One user

One reaction.

---

### 3. deleted_messages

Remember I suggested removing

```text
is_deleted_for_sender
```

Instead

```text
deleted_messages

id

created_at

message_id

user_id
```

Alice deletes.

Bob still sees it.

Perfect.

---

### 4. conversation_invites

Eventually

```text
Bob added Charlie
```

These are events.

I wouldn't build this now.

---

## Tier 3

---

### message_attachments

Probably the biggest table.

```text
id

created_at

message_id

type

storage_key

mime_type

size

thumbnail_key

metadata JSONB
```

One message

↓

Many attachments.

---

### live_locations

```text
id

message_id

latitude

longitude

expires_at
```

Later

stream updates.

---

### link_previews

```text
id

message_id

url

title

description

image_url
```

Generated asynchronously.

---

### message_forwards

If you later want

> forwarded many times

```text
id

original_message_id

forwarded_message_id
```

---

### message_pins

Since

> anyone can pin

I'd do

```text
id

message_id

user_id

created_at
```

instead of a boolean.

---

# Search

Notice

I intentionally did **not** create a search table.

Because PostgreSQL already supports

```text
GIN

tsvector
```

Later

If needed

move to

OpenSearch

or

Elasticsearch.

Don't design for that now.

---

# Presence
[[Presence]]
Don't store online/offline in PostgreSQL.

Keep it in

Redis

Memory

WebSocket Manager

because

Presence changes constantly.

---

# WebSocket connections

Don't create a database table.

Use Redis or in-memory maps.

```text
device_id

↓

connection
```

Never persist this.

---

# Notifications

Later

```text
device_push_tokens

id

device_id

provider

token
```

because

FCM

APNS

etc.

---

# Typing indicators

Again

Redis.

Not SQL.

---

# Blocked users

Since it's in your feature list.

```text
blocked_users

id

created_at

blocker_id

blocked_id
```

Unique

```text
(blocker_id,blocked_id)
```

---

# Message edits

This is optional.

If you don't care about edit history

Current schema is enough.

If you do

```text
message_edit_history

id

message_id

old_content

edited_at
```

---

# Group permissions

Later

```text
group_settings

conversation_id

only_admin_can_post

only_admin_can_add

disappearing_messages

...
```

Don't pollute

```text
conversation
```

---


---

# One thing I'd change philosophically

I noticed you're trying very hard to keep the schema "complete."

I would resist that temptation.

A good production schema is surprisingly **boring**.

Your current core tables are only:

- `users`
    
- `conversations`
    
- `conversation_participants`
    
- `messages`
    
- `devices`
    

That's excellent.

Everything else should feel like a plugin.

When you implement reactions, you add `message_reactions`.

When you implement attachments, you add `message_attachments`.

When you implement live locations, you add `live_locations`.