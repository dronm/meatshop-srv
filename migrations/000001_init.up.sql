BEGIN;

	CREATE TYPE role_types AS ENUM (
		'admin',
		'customer'
	);
					
	CREATE TYPE locales AS ENUM (
		'ru'
	);
	/* type get function */
	CREATE OR REPLACE FUNCTION enum_role_types_val(role_types,locales)
	RETURNS text AS $$
		SELECT
		CASE
		WHEN $1='admin'::role_types AND $2='ru'::locales THEN 'Администратор'
		WHEN $1='customer'::role_types AND $2='ru'::locales THEN 'Покупатель'
		ELSE ''
		END;		
	$$ LANGUAGE sql;	

-- users

	CREATE TABLE public.users
	(id serial NOT NULL,
	name  varchar(50) NOT NULL,
	role_id role_types NOT NULL,
	pwd  varchar(32),
	create_dt timestampTZ
			DEFAULT CURRENT_TIMESTAMP,
	banned bool
			DEFAULT FALSE,
	locale_id locales,CONSTRAINT users_pkey PRIMARY KEY (id)
	);
	CREATE UNIQUE INDEX users_name_index
	ON users(lower(name));

--logins
	CREATE TABLE public.logins
	(
		id serial NOT NULL,
		date_time_in timestampTZ,
		date_time_out timestampTZ,
		ip  varchar(15),
		session_id  varchar(128),
		user_id int,
		pub_key  varchar(15),
		set_date_time timestampTZ,
		headers jsonb,
		user_agent jsonb,
		CONSTRAINT logins_pkey PRIMARY KEY (id)
	);
	CREATE INDEX logins_session_idx
	ON logins(session_id);
	CREATE INDEX logins_user_idx
	ON logins(user_id);
	CREATE INDEX logins_pub_key_idx
	ON logins(pub_key);

--login devices
	CREATE TABLE public.login_device_bans
	(
		user_id int NOT NULL,
		hash  varchar(32) NOT NULL,
		create_dt timestampTZ DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT login_device_bans_pkey PRIMARY KEY (user_id,hash)
	);
--

-- permisssions
	CREATE TABLE permissions (
		code text PRIMARY KEY,
		description text NOT NULL DEFAULT ''
	);

	CREATE TABLE role_permissions (
		role_id role_types NOT NULL,
		permission_code text NOT NULL REFERENCES permissions(code) ON DELETE CASCADE,
		PRIMARY KEY (role_id, permission_code)
	);

	INSERT INTO permissions (code, description)
	VALUES
		('user.list', 'List users'),
		('user.detail', 'View user'),
		('user.update', 'Update user'),
		('user.delete', 'Delete user'),
		('user.logout', 'Logout user')
	ON CONFLICT (code) DO NOTHING;

	INSERT INTO role_permissions (role_id, permission_code)
	SELECT 'admin', code
	FROM permissions
	ON CONFLICT DO NOTHING;

COMMIT;

