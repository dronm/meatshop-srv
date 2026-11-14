BEGIN;

-- Keep editable message bodies and avoid duplicating an existing sale place field.
UPDATE public.notification_templates
SET body_template = body_template || E'\nТочка продажи: {{.SalePlaceName}}'
WHERE event = 'order.changed'
	AND strpos(body_template, '.SalePlaceName') = 0;

COMMIT;
