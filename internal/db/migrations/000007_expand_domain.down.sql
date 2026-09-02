-- Collapse additive roles for rollback compatibility. Highest privilege wins.
DROP TRIGGER IF EXISTS trg_users_registered_role ON users;
DROP FUNCTION IF EXISTS ensure_registered_user_role();

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS role VARCHAR(32) NOT NULL DEFAULT 'user'
    CHECK (role IN ('user', 'moderator', 'admin'));

UPDATE users AS u
SET role = selected.role
FROM (
    SELECT user_id,
           CASE
               WHEN bool_or(role = 'admin') THEN 'admin'
               WHEN bool_or(role = 'moderator') THEN 'moderator'
               ELSE 'user'
           END AS role
    FROM user_roles
    GROUP BY user_id
) AS selected
WHERE selected.user_id = u.id;

DROP TABLE IF EXISTS user_roles;
