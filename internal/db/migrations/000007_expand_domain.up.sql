-- Normalize authorization around explicit, persisted roles.
-- Every active account is a registered user; moderator/admin are additive roles.

CREATE TABLE IF NOT EXISTS user_roles (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role VARCHAR(32) NOT NULL CHECK (role IN ('user', 'moderator', 'admin')),
    granted_by UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role)
);

CREATE INDEX IF NOT EXISTS idx_user_roles_role ON user_roles (role, user_id);

-- Preserve staff values if an earlier development schema introduced users.role.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'users'
          AND column_name = 'role'
    ) THEN
        EXECUTE $sql$
            INSERT INTO user_roles (user_id, role)
            SELECT id,
                   CASE
                       WHEN role::text IN ('admin', 'moderator', 'user') THEN role::text
                       ELSE 'user'
                   END
            FROM users
            WHERE deleted_at IS NULL
            ON CONFLICT (user_id, role) DO NOTHING
        $sql$;
    END IF;
END $$;

INSERT INTO user_roles (user_id, role)
SELECT id, 'user'
FROM users
WHERE deleted_at IS NULL
ON CONFLICT (user_id, role) DO NOTHING;

CREATE OR REPLACE FUNCTION ensure_registered_user_role()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO user_roles (user_id, role)
    VALUES (NEW.id, 'user')
    ON CONFLICT (user_id, role) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_users_registered_role ON users;
CREATE TRIGGER trg_users_registered_role
AFTER INSERT ON users
FOR EACH ROW EXECUTE FUNCTION ensure_registered_user_role();

ALTER TABLE users DROP COLUMN IF EXISTS role;
