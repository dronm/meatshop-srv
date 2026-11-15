-- JSON null leaves the customer unconfigured until an administrator selects it.
-- Never guess a customer's ID or overwrite an administrator's existing value.
INSERT INTO public.webapp_constants (code, name, description, value_type, value)
VALUES (
	'cash_customer',
	'Покупатель за наличный расчёт',
	'Покупатель для заказов за наличный расчёт. Не задан, пока покупатель не выбран администратором.',
	'json',
	'null'::jsonb
)
ON CONFLICT (code) DO NOTHING;
