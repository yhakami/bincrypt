-- BinCrypt PostgreSQL Schema
-- Version: 1.0.0
-- Description: Initial schema with paste storage, deduplication, and ban lists

-- Pastes table: stores paste content and metadata
CREATE TABLE IF NOT EXISTS pastes (
    id VARCHAR(43) PRIMARY KEY,
    content TEXT,
    references_id VARCHAR(43),
    content_hash VARCHAR(64) NOT NULL,
    plaintext_hash VARCHAR(64),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    burn_after_read BOOLEAN NOT NULL DEFAULT FALSE,
    size_bytes INTEGER NOT NULL,
    is_plaintext BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Hash index: maps content hash to original paste ID for deduplication
CREATE TABLE IF NOT EXISTS hash_index (
    content_hash VARCHAR(64) PRIMARY KEY,
    paste_id VARCHAR(43) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Burned pastes: tracks burn-after-read pastes that have been viewed
CREATE TABLE IF NOT EXISTS burned_pastes (
    paste_id VARCHAR(43) PRIMARY KEY,
    burned_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Banned hashes: block content by ciphertext or plaintext hash
CREATE TABLE IF NOT EXISTS banned_hashes (
    hash VARCHAR(64) PRIMARY KEY,
    hash_type VARCHAR(20) NOT NULL CHECK (hash_type IN ('ciphertext', 'plaintext')),
    reason TEXT NOT NULL,
    banned_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_pastes_expires_at ON pastes(expires_at);
CREATE INDEX IF NOT EXISTS idx_pastes_content_hash ON pastes(content_hash);
CREATE INDEX IF NOT EXISTS idx_pastes_references_id ON pastes(references_id) WHERE references_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_banned_hashes_type ON banned_hashes(hash_type);

-- Foreign key constraints
ALTER TABLE pastes ADD CONSTRAINT fk_pastes_references
    FOREIGN KEY (references_id) REFERENCES pastes(id) ON DELETE SET NULL;

ALTER TABLE hash_index ADD CONSTRAINT fk_hash_index_paste
    FOREIGN KEY (paste_id) REFERENCES pastes(id) ON DELETE CASCADE;

ALTER TABLE burned_pastes ADD CONSTRAINT fk_burned_pastes_paste
    FOREIGN KEY (paste_id) REFERENCES pastes(id) ON DELETE CASCADE;

-- Automatic cleanup function for expired pastes
CREATE OR REPLACE FUNCTION cleanup_expired_pastes() RETURNS void AS $$
BEGIN
    DELETE FROM pastes WHERE expires_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- Optional: Create scheduled job to cleanup expired pastes (requires pg_cron extension)
-- SELECT cron.schedule('cleanup-expired-pastes', '0 * * * *', 'SELECT cleanup_expired_pastes()');

-- Comments for documentation
COMMENT ON TABLE pastes IS 'Stores encrypted paste content and metadata';
COMMENT ON TABLE hash_index IS 'Maps content hashes to original paste IDs for deduplication';
COMMENT ON TABLE burned_pastes IS 'Tracks pastes that have been burned (viewed once)';
COMMENT ON TABLE banned_hashes IS 'Stores banned content hashes (ciphertext or plaintext)';
COMMENT ON COLUMN pastes.content IS 'Base64-encoded encrypted content (NULL for reference pastes)';
COMMENT ON COLUMN pastes.references_id IS 'Points to original paste ID if deduplicated';
COMMENT ON COLUMN pastes.content_hash IS 'SHA-256 hash of ciphertext for deduplication';
COMMENT ON COLUMN pastes.plaintext_hash IS 'SHA-256 hash of plaintext content (for unencrypted pastes)';
