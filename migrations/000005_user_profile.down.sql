BEGIN;

DELETE FROM role_permissions
WHERE role_id <> 'admin'::role_types
	AND permission_code IN (
		'user.logout',
		'mainMenu.for_user',
		'progAbout.info'
	);

DELETE FROM permissions
WHERE code IN (
	'user.profile.detail',
	'user.profile.update',
	'user.profile.password'
);

COMMIT;
