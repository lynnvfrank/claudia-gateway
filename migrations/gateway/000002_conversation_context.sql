-- Semantic conversation merge + rolling fingerprint state (see conversation_merge config).

CREATE TABLE IF NOT EXISTS conversation_context (
  conversation_id TEXT NOT NULL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  project_id TEXT NOT NULL DEFAULT '',
  flavor_id TEXT NOT NULL DEFAULT '',
  last_user_embedding BLOB NOT NULL,
  embedding_dim INTEGER NOT NULL,
  last_user_text_normalized TEXT NOT NULL DEFAULT '',
  last_model_text_normalized TEXT NOT NULL DEFAULT '',
  last_updated_unix REAL NOT NULL,
  rolling_fingerprint TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_conversation_context_scope_time
  ON conversation_context (tenant_id, project_id, flavor_id, last_updated_unix DESC);

-- Short-lived JSON completion cache for duplicate HTTP retries (same scope + fingerprint + user text).
CREATE TABLE IF NOT EXISTS conversation_dedup_cache (
  dedup_key TEXT NOT NULL PRIMARY KEY,
  response_body BLOB NOT NULL,
  created_unix REAL NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_conversation_dedup_created ON conversation_dedup_cache (created_unix);
