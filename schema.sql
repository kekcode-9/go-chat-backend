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
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sender_id UUID NOT NULL REFERENCES users(id),
    conversation_id UUID NOT NULL REFERENCES conversations(id),
    content TEXT NOT NULL,
    deleted_at TIMESTAMPTZ,
    deleted_by UUID REFERENCES users(id),
    updated_at TIMESTAMPTZ,
    reply_to_message_id UUID,
    forwarded_from_message UUID REFERENCES messages(id),
    forwarded_by UUID REFERENCES users(id),
    sequence_no BIGINT NOT NULL DEFAULT 1,
    client_message_id UUID,
    UNIQUE (conversation_id, sequence_no),
    UNIQUE (sender_id, client_message_id),
    CONSTRAINT messages_conversation_id_id_key UNIQUE (conversation_id, id),
    CONSTRAINT messages_reply_to_conversation_message_fkey
        FOREIGN KEY (conversation_id, reply_to_message_id)
        REFERENCES messages (conversation_id, id)
);

CREATE INDEX idx_messages_conversation_sequence ON messages (conversation_id, sequence_no);

CREATE INDEX idx_messages_sender ON messages (sender_id);

CREATE INDEX idx_messages_conversation_reply_sequence
    ON messages (conversation_id, reply_to_message_id, sequence_no)
    WHERE reply_to_message_id IS NOT NULL;

-- =============================================================================
-- Conversation Participants
-- =============================================================================

CREATE TABLE conversation_participants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'admin')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'left', 'removed')),
    conversation_id UUID NOT NULL REFERENCES conversations(id),
    user_id UUID NOT NULL REFERENCES users(id),
    left_at TIMESTAMPTZ,
    removed_at TIMESTAMPTZ,
    removed_by UUID REFERENCES users(id),
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
    session_id UUID NOT NULL,
    refresh_token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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

CREATE TABLE message_reactions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    conversation_id UUID NOT NULL,
    message_id  UUID NOT NULL,
    user_id     UUID NOT NULL REFERENCES users(id),
    reaction    TEXT NOT NULL,

    UNIQUE (message_id, user_id),
    CONSTRAINT message_reactions_conversation_message_fkey
        FOREIGN KEY (conversation_id, message_id)
        REFERENCES messages (conversation_id, id)
);

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

CREATE TABLE live_locations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL,
    message_id  UUID NOT NULL,
    latitude    DOUBLE PRECISION NOT NULL,
    longitude   DOUBLE PRECISION NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT live_locations_conversation_message_fkey
        FOREIGN KEY (conversation_id, message_id)
        REFERENCES messages (conversation_id, id)
);

CREATE TABLE link_previews (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL,
    message_id  UUID NOT NULL,
    url         TEXT NOT NULL,
    title       TEXT NOT NULL,
    description TEXT,
    image_url   TEXT,
    CONSTRAINT link_previews_conversation_message_fkey
        FOREIGN KEY (conversation_id, message_id)
        REFERENCES messages (conversation_id, id)
);

CREATE TABLE message_pins (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL,
    message_id  UUID NOT NULL,
    user_id     UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT message_pins_conversation_message_fkey
        FOREIGN KEY (conversation_id, message_id)
        REFERENCES messages (conversation_id, id)
);

CREATE TABLE message_edit_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL,
    message_id  UUID NOT NULL,
    old_content TEXT NOT NULL,
    edited_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT message_edit_history_conversation_message_fkey
        FOREIGN KEY (conversation_id, message_id)
        REFERENCES messages (conversation_id, id)
);

CREATE TABLE group_settings (
    conversation_id       UUID PRIMARY KEY REFERENCES conversations(id),
    only_admin_can_post   BOOLEAN NOT NULL DEFAULT FALSE,
    only_admin_can_add    BOOLEAN NOT NULL DEFAULT FALSE,
    disappearing_messages  BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE device_push_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id   UUID NOT NULL REFERENCES devices(id),
    provider    TEXT NOT NULL,
    token       TEXT NOT NULL
);
