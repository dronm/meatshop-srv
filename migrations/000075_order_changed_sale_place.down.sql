BEGIN;

-- Template bodies are editable content. Keep the added sale place line when
-- rolling back so existing or subsequently customized message text is preserved.
-- It can be removed explicitly in the notification template editor if needed.

COMMIT;
