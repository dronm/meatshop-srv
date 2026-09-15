BEGIN;

CREATE TABLE public.application_routes (
	id serial PRIMARY KEY,
	name text NOT NULL UNIQUE,
	path text NOT NULL,
	descr varchar(250) NOT NULL,
	section varchar(250) NOT NULL,
	icon varchar(250),
	menu_available boolean NOT NULL DEFAULT false,
	is_active boolean NOT NULL DEFAULT true,
	created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE public.main_menus (
	id serial PRIMARY KEY,
	role_id role_types,
	user_id integer REFERENCES public.users(id) ON DELETE CASCADE,
	parent_id integer REFERENCES public.main_menus(id) ON DELETE CASCADE,
	caption varchar(250) NOT NULL,
	route_id integer REFERENCES public.application_routes(id) ON DELETE SET NULL,
	icon varchar(250),
	sort_order integer NOT NULL DEFAULT 0,
	is_active boolean NOT NULL DEFAULT true,
	created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT main_menus_owner_check CHECK ((role_id IS NULL) <> (user_id IS NULL))
);

CREATE INDEX main_menus_role_idx ON public.main_menus(role_id);
CREATE INDEX main_menus_user_idx ON public.main_menus(user_id);
CREATE INDEX main_menus_parent_idx ON public.main_menus(parent_id);

CREATE OR REPLACE VIEW public.main_menus_list AS
SELECT
	m.id,
	m.role_id,
	CASE
		WHEN u.id IS NULL THEN NULL
		ELSE jsonb_build_object(
			'keys', jsonb_build_object('id', u.id),
			'descr', u.name
		)
	END AS users_ref,
	CASE
		WHEN parent.id IS NULL THEN NULL
		ELSE jsonb_build_object(
			'keys', jsonb_build_object('id', parent.id),
			'descr', parent.caption
		)
	END AS main_menus_ref,
	CASE
		WHEN ar.id IS NULL THEN NULL
		ELSE jsonb_build_object(
			'keys', jsonb_build_object('id', ar.id),
			'descr', ar.descr
		)
	END AS application_routes_ref,
	ar.name AS route_name,
	m.caption,
	m.icon,
	m.sort_order,
	m.is_active,
	m.created_at,
	m.updated_at
FROM public.main_menus AS m
LEFT JOIN public.users AS u ON u.id = m.user_id
LEFT JOIN public.main_menus AS parent ON parent.id = m.parent_id
LEFT JOIN public.application_routes AS ar ON ar.id = m.route_id;

INSERT INTO permissions (code, description)
VALUES
	('user.create', 'Create user'),
	('mainMenu.create', 'Create main menu item'),
	('mainMenu.list', 'List main menu items'),
	('mainMenu.for_user', 'Load main menu for logged user'),
	('mainMenu.routeAutocomplete', 'Search application routes for main menu'),
	('mainMenu.detail', 'View main menu item'),
	('mainMenu.update', 'Update main menu item'),
	('mainMenu.delete', 'Delete main menu item'),
	('applicationRoute.list', 'List frontend application routes'),
	('applicationRoute.detail', 'View frontend application route'),
	('applicationRoute.update', 'Update frontend application route'),
	('applicationRoute.sync', 'Synchronize frontend application routes'),
	('progAbout.info', 'View application information')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_code)
SELECT 'admin', code
FROM permissions
ON CONFLICT DO NOTHING;

INSERT INTO public.application_routes (
	name,
	path,
	descr,
	section,
	icon,
	menu_available
)
VALUES (
	'users',
	'/users',
	'Пользователи',
	'Администрирование',
	'pi pi-users',
	true
)
ON CONFLICT (name) DO UPDATE
SET
	path = EXCLUDED.path,
	descr = EXCLUDED.descr,
	section = EXCLUDED.section,
	icon = EXCLUDED.icon,
	menu_available = EXCLUDED.menu_available,
	is_active = true;

INSERT INTO public.main_menus (
	role_id,
	caption,
	route_id,
	icon,
	sort_order,
	is_active
)
SELECT
	'admin'::role_types,
	'Пользователи',
	ar.id,
	'pi pi-users',
	10,
	true
FROM public.application_routes AS ar
WHERE ar.name = 'users'
	AND NOT EXISTS (
		SELECT 1
		FROM public.main_menus AS m
		WHERE m.role_id = 'admin'::role_types
			AND m.user_id IS NULL
			AND m.route_id = ar.id
	);

COMMIT;
