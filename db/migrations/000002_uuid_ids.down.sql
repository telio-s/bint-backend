ALTER TABLE oauth_identities
    DROP CONSTRAINT oauth_identities_user_id_fkey,
    ALTER COLUMN id DROP DEFAULT;

ALTER TABLE refresh_tokens
    DROP CONSTRAINT refresh_tokens_user_id_fkey,
    ALTER COLUMN id DROP DEFAULT;

ALTER TABLE users
    ALTER COLUMN id DROP DEFAULT;

ALTER TABLE oauth_identities
    ALTER COLUMN id TYPE TEXT USING id::text,
    ALTER COLUMN user_id TYPE TEXT USING user_id::text;

ALTER TABLE refresh_tokens
    ALTER COLUMN id TYPE TEXT USING id::text,
    ALTER COLUMN user_id TYPE TEXT USING user_id::text;

ALTER TABLE users
    ALTER COLUMN id TYPE TEXT USING id::text;

ALTER TABLE oauth_identities
    ADD CONSTRAINT oauth_identities_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE refresh_tokens
    ADD CONSTRAINT refresh_tokens_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;
