BEGIN;

INSERT INTO permissions (code, description)
VALUES
	('user.profile.detail', 'View own user profile'),
	('user.profile.update', 'Update own user profile'),
	('user.profile.password', 'Change own user password')
ON CONFLICT (code) DO NOTHING;

-- These are common authenticated-user capabilities, not administrator-only
-- capabilities. Grant them, together with the shell operations needed after
-- login, to every role currently defined by role_types.
INSERT INTO role_permissions (role_id, permission_code)
SELECT
	r.role_id,
	p.permission_code
FROM unnest(enum_range(NULL::role_types)) AS r(role_id)
CROSS JOIN (
	VALUES
		('user.profile.detail'),
		('user.profile.update'),
		('user.profile.password'),
		('user.logout'),
		('mainMenu.for_user'),
		('progAbout.info')
) AS p(permission_code)
ON CONFLICT DO NOTHING;

COMMIT;
