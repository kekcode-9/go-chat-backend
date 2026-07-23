-- +goose Up
SELECT
    'up SQL query';

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- =============================================================================
-- Users
-- =============================================================================
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_name TEXT NOT NULL,
    email TEXT UNIQUE,
    password TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- Conversations
-- =============================================================================
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    type TEXT NOT NULL DEFAULT 'direct' CHECK (type IN ('direct', 'group')),
    title TEXT,
    description TEXT,
    avatar_url TEXT,
    created_by UUID NOT NULL REFERENCES users(id),
    next_sequence_no BIGINT NOT NULL DEFAULT 1
);

-- =============================================================================
-- Messages
-- (created before conversation_participants so the FK on last_read_message_id works)
-- =============================================================================
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sender_id UUID NOT NULL REFERENCES users(id),
    conversation_id UUID NOT NULL REFERENCES conversations(id),
    content TEXT NOT NULL,
    deleted_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    reply_to_message_id UUID REFERENCES messages(id),
    sequence_no BIGINT NOT NULL DEFAULT 1,
    client_message_id UUID UNIQUE,
    UNIQUE (conversation_id, sequence_no)
);

CREATE INDEX idx_messages_conversation_sequence ON messages (conversation_id, sequence_no);

CREATE INDEX idx_messages_sender ON messages (sender_id);

-- =============================================================================
-- Conversation Participants
-- =============================================================================
CREATE TABLE conversation_participants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'admin')),
    conversation_id UUID NOT NULL REFERENCES conversations(id),
    user_id UUID NOT NULL REFERENCES users(id),
    left_at TIMESTAMPTZ,
    is_muted BOOLEAN NOT NULL DEFAULT FALSE,
    is_archived BOOLEAN NOT NULL DEFAULT FALSE,
    is_pinned BOOLEAN NOT NULL DEFAULT FALSE,
    last_read_message_id UUID REFERENCES messages(id),
    last_read_mssg_seq BIGINT,
    UNIQUE (conversation_id, user_id)
);

CREATE INDEX idx_conv_participants_user ON conversation_participants (user_id);

-- =============================================================================
-- Devices
-- =============================================================================
CREATE TABLE devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    device_name TEXT NOT NULL,
    device_type TEXT NOT NULL,
    public_key TEXT,
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_devices_user ON devices (user_id);

-- =============================================================================
-- Refresh_sessions
-- =============================================================================

CREATE TABLE refresh_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    device_id UUID NOT NULL REFERENCES devices(id),
    refresh_token_hash TEXT NOT NULL,
    expires_At TIMESTAMPTZ NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW() last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)

-- =============================================================================
-- Blocked Users
-- =============================================================================
CREATE TABLE blocked_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    blocker_id UUID NOT NULL REFERENCES users(id),
    blocked_id UUID NOT NULL REFERENCES users(id),
    UNIQUE (blocker_id, blocked_id)
);

-- +goose Down
DROP TABLE IF EXISTS refresh_sessions;

DROP TABLE IF EXISTS blocked_users;

DROP TABLE IF EXISTS devices;

DROP TABLE IF EXISTS conversation_participants;

DROP TABLE IF EXISTS messages;

DROP TABLE IF EXISTS conversations;

DROP TABLE IF EXISTS users;

DROP EXTENSION IF EXISTS "pgcrypto";