BEGIN;

	DROP FUNCTION enum_role_types_val(role_types,locales);
	DROP TYPE role_types;
	DROP TYPE locales;

	DROP TABLE public.users;
	DROP TABLE public.logins;

	DROP TABLE public.login_device_bans;

COMMIT;

