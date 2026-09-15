BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code IN (
	'notificationTemplate.create',
	'notificationTemplate.list',
	'notificationTemplate.detail',
	'notificationTemplate.update',
	'notificationTemplate.delete',
	'notificationTemplateRecipient.create',
	'notificationTemplateRecipient.list',
	'notificationTemplateRecipient.detail',
	'notificationTemplateRecipient.update',
	'notificationTemplateRecipient.delete'
);

DELETE FROM public.permissions
WHERE code IN (
	'notificationTemplate.create',
	'notificationTemplate.list',
	'notificationTemplate.detail',
	'notificationTemplate.update',
	'notificationTemplate.delete',
	'notificationTemplateRecipient.create',
	'notificationTemplateRecipient.list',
	'notificationTemplateRecipient.detail',
	'notificationTemplateRecipient.update',
	'notificationTemplateRecipient.delete'
);

DROP VIEW IF EXISTS public.notification_template_recipients_detail;
DROP VIEW IF EXISTS public.notification_template_recipients_list;
DROP FUNCTION IF EXISTS public.notification_templates_ref(public.notification_templates);
DROP TABLE IF EXISTS public.notification_template_recipients;
DROP TABLE IF EXISTS public.notification_templates;

COMMIT;
