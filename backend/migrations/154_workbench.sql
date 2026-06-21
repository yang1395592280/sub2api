CREATE TABLE IF NOT EXISTS workbench_conversations (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(160) NOT NULL DEFAULT '新会话',
    mode VARCHAR(16) NOT NULL DEFAULT 'chat',
    api_key_id BIGINT NULL REFERENCES api_keys(id) ON DELETE SET NULL,
    endpoint VARCHAR(64) NOT NULL DEFAULT 'chat_completions',
    model VARCHAR(200) NOT NULL DEFAULT '',
    last_message_preview VARCHAR(300) NOT NULL DEFAULT '',
    last_error VARCHAR(500) NULL,
    message_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_workbench_conversations_user_id
    ON workbench_conversations(user_id);
CREATE INDEX IF NOT EXISTS idx_workbench_conversations_user_updated
    ON workbench_conversations(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_workbench_conversations_deleted_at
    ON workbench_conversations(deleted_at);

CREATE TABLE IF NOT EXISTS workbench_messages (
    id BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES workbench_conversations(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode VARCHAR(16) NOT NULL,
    role VARCHAR(16) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    api_key_id BIGINT NULL REFERENCES api_keys(id) ON DELETE SET NULL,
    endpoint VARCHAR(64) NOT NULL DEFAULT '',
    model VARCHAR(200) NOT NULL DEFAULT '',
    request_options JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    image_outputs JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(16) NOT NULL DEFAULT 'success',
    error_message VARCHAR(500) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_workbench_messages_conversation_created
    ON workbench_messages(conversation_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_workbench_messages_user_created
    ON workbench_messages(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_workbench_messages_status
    ON workbench_messages(status);
CREATE INDEX IF NOT EXISTS idx_workbench_messages_deleted_at
    ON workbench_messages(deleted_at);
