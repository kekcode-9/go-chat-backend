CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- =============================================================================
-- Users
-- =============================================================================

CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_name  TEXT NOT NULL,
    email      TEXT UNIQUE,
    password   TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- Conversations
-- =============================================================================

CREATE TABLE conversations (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    type              TEXT NOT NULL DEFAULT 'direct' CHECK (type IN ('direct', 'group')),
    title             TEXT,
    description       TEXT,
    avatar_url        TEXT,
    created_by        UUID NOT NULL REFERENCES users(id),
    next_sequence_no  BIGINT NOT NULL DEFAULT 1
);

-- =============================================================================
-- Messages
-- (created before conversation_participants so the FK on last_read_message_id works)
-- =============================================================================

CREATE TABLE messages (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sender_id            UUID NOT NULL REFERENCES users(id),
    conversation_id      UUID NOT NULL REFERENCES conversations(id),
    content              TEXT NOT NULL,
    deleted_at           TIMESTAMPTZ,
    updated_at           TIMESTAMPTZ,
    reply_to_message_id  UUID REFERENCES messages(id),
    sequence_no          BIGINT NOT NULL DEFAULT 1,
    client_message_id    UUID UNIQUE,

    UNIQUE (conversation_id, sequence_no)
);

CREATE INDEX idx_messages_conversation_sequence ON messages (conversation_id, sequence_no);
CREATE INDEX idx_messages_sender                ON messages (sender_id);

-- =============================================================================
-- Conversation Participants
-- =============================================================================

CREATE TABLE conversation_participants (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    role                  TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'admin')),
    conversation_id       UUID NOT NULL REFERENCES conversations(id),
    user_id               UUID NOT NULL REFERENCES users(id),
    left_at               TIMESTAMPTZ,
    is_muted              BOOLEAN NOT NULL DEFAULT FALSE,
    is_archived           BOOLEAN NOT NULL DEFAULT FALSE,
    is_pinned             BOOLEAN NOT NULL DEFAULT FALSE,
    last_read_message_id  UUID REFERENCES messages(id),
    last_read_mssg_seq    BIGINT,

    UNIQUE (conversation_id, user_id)
);

CREATE INDEX idx_conv_participants_user ON conversation_participants (user_id);

-- =============================================================================
-- Devices
-- =============================================================================

CREATE TABLE devices (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id),
    device_name  TEXT NOT NULL,
    device_type  TEXT NOT NULL,
    public_key   TEXT,
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at   TIMESTAMPTZ
);

CREATE INDEX idx_devices_user ON devices (user_id);

-- =============================================================================
-- Blocked Users
-- =============================================================================

CREATE TABLE blocked_users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    blocker_id  UUID NOT NULL REFERENCES users(id),
    blocked_id  UUID NOT NULL REFERENCES users(id),

    UNIQUE (blocker_id, blocked_id)
);

-- =============================================================================
-- Message Receipts (Tier 2)
-- =============================================================================

CREATE TABLE message_receipts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    message_id  UUID NOT NULL REFERENCES messages(id),
    user_id     UUID NOT NULL REFERENCES users(id),
    status      TEXT NOT NULL CHECK (status IN ('SENT', 'DELIVERED', 'READ')),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- Message Reactions (Tier 2)
-- =============================================================================

CREATE TABLE message_reactions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    message_id  UUID NOT NULL REFERENCES messages(id),
    user_id     UUID NOT NULL REFERENCES users(id),
    reaction    TEXT NOT NULL,

    UNIQUE (message_id, user_id)
);

-- =============================================================================
-- Deleted Messages (Tier 2)
-- =============================================================================

CREATE TABLE deleted_messages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    message_id  UUID NOT NULL REFERENCES messages(id),
    user_id     UUID NOT NULL REFERENCES users(id)
);

-- =============================================================================
-- Message Attachments (Tier 3)
-- =============================================================================

CREATE TABLE message_attachments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    message_id    UUID NOT NULL REFERENCES messages(id),
    type          TEXT NOT NULL,
    storage_key   TEXT NOT NULL,
    mime_type     TEXT NOT NULL,
    size          BIGINT NOT NULL,
    thumbnail_key TEXT,
    metadata      JSONB
);

-- =============================================================================
-- Live Locations (Tier 3)
-- =============================================================================

CREATE TABLE live_locations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id  UUID NOT NULL REFERENCES messages(id),
    latitude    DOUBLE PRECISION NOT NULL,
    longitude   DOUBLE PRECISION NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL
);

-- =============================================================================
-- Link Previews (Tier 3)
-- =============================================================================

CREATE TABLE link_previews (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id  UUID NOT NULL REFERENCES messages(id),
    url         TEXT NOT NULL,
    title       TEXT NOT NULL,
    description TEXT,
    image_url   TEXT
);

-- =============================================================================
-- Message Forwards (Tier 3)
-- =============================================================================

CREATE TABLE message_forwards (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_message_id  UUID NOT NULL REFERENCES messages(id),
    forwarded_message_id UUID NOT NULL REFERENCES messages(id)
);

-- =============================================================================
-- Message Pins (Tier 3)
-- =============================================================================

CREATE TABLE message_pins (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id  UUID NOT NULL REFERENCES messages(id),
    user_id     UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- Message Edit History (Tier 3)
-- =============================================================================

CREATE TABLE message_edit_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id  UUID NOT NULL REFERENCES messages(id),
    old_content TEXT NOT NULL,
    edited_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- Group Settings (Tier 3)
-- =============================================================================

CREATE TABLE group_settings (
    conversation_id       UUID PRIMARY KEY REFERENCES conversations(id),
    only_admin_can_post   BOOLEAN NOT NULL DEFAULT FALSE,
    only_admin_can_add    BOOLEAN NOT NULL DEFAULT FALSE,
    disappearing_messages  BOOLEAN NOT NULL DEFAULT FALSE
);

-- =============================================================================
-- Device Push Tokens (Tier 3)
-- =============================================================================

CREATE TABLE device_push_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id   UUID NOT NULL REFERENCES devices(id),
    provider    TEXT NOT NULL,
    token       TEXT NOT NULL
);
