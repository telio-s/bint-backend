-- name: CreateEmailUser :one
INSERT INTO users (
    email, display_name, avatar_url, email_verified, password_hash, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING id, email, display_name, avatar_url, email_verified, created_at, updated_at;

-- name: CreateGoogleUser :one
INSERT INTO users (
    email, display_name, avatar_url, email_verified, password_hash, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, NULL, $5, $6
)
RETURNING id, email, display_name, avatar_url, email_verified, created_at, updated_at;

-- name: GetUserByID :one
SELECT id, email, display_name, avatar_url, email_verified, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, display_name, avatar_url, email_verified, created_at, updated_at
FROM users
WHERE email = $1;

-- name: UpdateUserFromGoogle :one
UPDATE users
SET email_verified = TRUE,
    display_name = CASE WHEN display_name = '' THEN $2 ELSE display_name END,
    avatar_url = CASE WHEN avatar_url = '' THEN $3 ELSE avatar_url END,
    updated_at = $4
WHERE id = $1
RETURNING id, email, display_name, avatar_url, email_verified, created_at, updated_at;

-- name: GetUserWithPasswordByEmail :one
SELECT id, email, display_name, avatar_url, email_verified, password_hash, created_at, updated_at
FROM users
WHERE email = $1 AND password_hash IS NOT NULL;

-- name: GetUserByGoogleSubject :one
SELECT u.id, u.email, u.display_name, u.avatar_url, u.email_verified, u.created_at, u.updated_at
FROM users AS u
JOIN oauth_identities AS oi ON oi.user_id = u.id
WHERE oi.provider = 'google' AND oi.provider_subject = $1;

-- name: CreateGoogleIdentity :exec
INSERT INTO oauth_identities (user_id, provider, provider_subject, created_at)
VALUES ($1, 'google', $2, $3);

-- name: StoreRefreshToken :exec
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at)
VALUES ($1, $2, $3, $4);

-- name: GetRefreshTokenForUpdate :one
SELECT u.id, u.email, u.display_name, u.avatar_url, u.email_verified, u.created_at, u.updated_at
FROM refresh_tokens AS rt
JOIN users AS u ON u.id = rt.user_id
WHERE rt.token_hash = $1
  AND rt.revoked_at IS NULL
  AND rt.expires_at > NOW()
FOR UPDATE OF rt;

-- name: RevokeRefreshToken :execrows
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE token_hash = $1 AND revoked_at IS NULL;
