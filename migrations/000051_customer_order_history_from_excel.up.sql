-- Import historical customer product selections from Клиенты+что покупают.xlsx.
--
-- The workbook contains 84 sale-place rows and 14 merged product-list ranges.
-- Merged values are expanded below so each source row is self-contained.
-- Customers are resolved by INN. Sale places are resolved by customer and
-- address; Заведение is used only as a tie-breaker when an address is shared.
--
-- Workbook product names are legacy abbreviations. The explicit alias table
-- maps all 31 normalized source names to products seeded by migration 000041.
-- Temperature-unspecified Желудки, Сердце, and Фарш are treated as chilled.

BEGIN;

CREATE TEMPORARY TABLE migration_order_history_source (
	source_row integer PRIMARY KEY,
	place_name text NOT NULL,
	inn varchar(12) NOT NULL,
	address text NOT NULL,
	product_names text[] NOT NULL CHECK (cardinality(product_names) > 0)
) ON COMMIT DROP;

INSERT INTO migration_order_history_source (
	source_row,
	place_name,
	inn,
	address,
	product_names
)
VALUES
	(2, 'Ассорти', '723001750170', 'Тюменская обл., г. Тюмень, Луначарского ул., д. 12', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Филе бедра ц/б с кожей охлажденное', 'Окорочок ц/б на кости охлажденный', 'Яйцо куриное С1', 'Печень ц/б замороженная']::text[]),
	(3, 'Ассорти', '723001750170', 'Тюменская обл., г. Тюмень, Котовского ул., д. 55 к. 1', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Филе бедра ц/б с кожей охлажденное', 'Окорочок ц/б на кости охлажденный', 'Яйцо куриное С1', 'Печень ц/б замороженная']::text[]),
	(4, 'Ассорти', '723001750170', 'Тюменская обл., г. Тюмень, 50 лет Октября ул., д. 14, пом.35 по 45', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Филе бедра ц/б с кожей охлажденное', 'Окорочок ц/б на кости охлажденный', 'Яйцо куриное С1', 'Печень ц/б замороженная']::text[]),
	(5, 'Ассорти', '723001750170', 'Тюменская обл., г. Тюмень, Республики ул., д.5А', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Филе бедра ц/б с кожей охлажденное', 'Окорочок ц/б на кости охлажденный', 'Яйцо куриное С1', 'Печень ц/б замороженная']::text[]),
	(6, 'Ассорти', '723001750170', 'Тюменская обл., г. Тюмень, Пермякова ул., д.1)', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Филе бедра ц/б с кожей охлажденное', 'Окорочок ц/б на кости охлажденный', 'Яйцо куриное С1', 'Печень ц/б замороженная']::text[]),
	(7, 'Ассорти', '723001750170', 'Тюменская обл., г. Тюмень, 2-й км Старо-Тобольского тракта км, д. 8, стр. 111', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Филе бедра ц/б с кожей охлажденное', 'Окорочок ц/б на кости охлажденный', 'Яйцо куриное С1', 'Печень ц/б замороженная']::text[]),
	(8, 'Ассорти', '723001750170', 'Тюменская обл., г. Тюмень, Мельникайте ул., д. 70', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Филе бедра ц/б с кожей охлажденное', 'Окорочок ц/б на кости охлажденный', 'Яйцо куриное С1', 'Печень ц/б замороженная']::text[]),
	(9, 'Донер на углях', '860236075624', 'Тюменская обл., г. Тюмень, Челюскинцев ул., д. 50', ARRAY['Филе грудки ц/б охлажденное']::text[]),
	(10, 'Баста', '7840107432', 'Тюменская обл., г. Тюмень, Республики ул., д. 42', ARRAY['Филе бедра ц/б охлажденное', 'Крылья ц/б 2ф замороженные', 'Филе грудки ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Крылья ц/б 2ф охлажденные']::text[]),
	(11, 'Борщ и пельмени', '720212438207', 'Тюменская обл., г. Тюмень, Одесская ул., д. 3, стр. 1', ARRAY['Филе грудки ц/б охлажденное']::text[]),
	(12, 'Брют', '7453355050', 'Тюменская обл., г. Тюмень, 50 лет Октября ул., д. 44 помещение 2', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Крылья ц/б 3ф охлажденные']::text[]),
	(13, 'Брют', '7203583142', 'Тюменская обл., г. Тюмень, Тихий проезд, д. 6 помещение 1', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Крылья ц/б 3ф охлажденные']::text[]),
	(14, 'Брют', '7453364182', 'Тюменская обл., г. Тюмень, Мельникайте ул., д. 116 к. 1', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Крылья ц/б 3ф охлажденные']::text[]),
	(15, 'Брют', '7453364182', 'Тюменская обл., г. Тюмень, Советская ул., д. 55, помещ. 1', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Крылья ц/б 3ф охлажденные']::text[]),
	(16, 'Брют', '7453364182', 'Тюменская обл., г. Тюмень, Ленина ул., д. 57, помещ. 2)', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Крылья ц/б 3ф охлажденные']::text[]),
	(17, 'Бублик', '2635258077', 'Тюменская обл., г. Тюмень, Мельникайте ул., д. 116 стр. 1', ARRAY['Крылья ц/б 2ф охлажденные', 'Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Печень ц/б замороженная', 'Бедро ц/б охлажденное']::text[]),
	(18, 'Бублик', '2635258077', 'Тюменская обл., г. Тюмень, Александра Логунова ул., д. 5а, пом.77', ARRAY['Крылья ц/б 2ф охлажденные', 'Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Печень ц/б замороженная', 'Бедро ц/б охлажденное']::text[]),
	(19, 'Бульвар бар', '7203578103', 'Тюменская обл., г. Тюмень, Герцена ул., д. 63', ARRAY['Филе бедра ц/б охлажденное', 'Филе грудки ц/б охлажденное', 'Яйцо куриное С0 мытое дезинфицированное', 'Тушка Ц/Б 1 сорт охлажденная']::text[]),
	(20, 'Хурма', '7203578103', 'Тюменская обл., г. Тюмень, Ленина ул., д. 56В', ARRAY['Филе бедра ц/б охлажденное', 'Филе грудки ц/б охлажденное', 'Яйцо куриное С0 мытое дезинфицированное', 'Тушка Ц/Б 1 сорт охлажденная']::text[]),
	(21, 'Водокачка', '7203477458', 'Тюменская обл., г. Тюмень, 25 Октября ул., д. 23б', ARRAY['Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная']::text[]),
	(22, 'Грузинка', '7203243481', 'Тюменская обл., г. Тюмень, Володарского ул., д. 20', ARRAY['Филе бедра ц/б охлажденное', 'Крылья ц/б 2ф охлажденные', 'Филе грудки ц/б охлажденное', 'Тушка ц/б 1 сорт охлажденная', 'Яйцо куриное С1']::text[]),
	(23, 'Узбечка', '7203243481', 'Тюменская обл., г. Тюмень, Перекопская ул., д. 4А/2А', ARRAY['Филе бедра ц/б охлажденное', 'Крылья ц/б 2ф охлажденные', 'Филе грудки ц/б охлажденное', 'Тушка ц/б 1 сорт охлажденная', 'Яйцо куриное С1', 'Окорочок ц/б на кости охлажденный']::text[]),
	(24, 'Гуд Фуд', '7203566500', 'Тюменская обл., г. Тюмень, Тимофея Чаркова ул., д. 43', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное']::text[]),
	(25, 'Мясник и море (ДНКфуд)', '5406981628', 'Тюменская обл., г. Тюмень, Сергея Ильюшина ул., д.10', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Голень ц/б замороженная', 'Фарш из ц/б филейный']::text[]),
	(26, 'Ермолаев', '7203495640', 'Тюменская обл., г. Тюмень, Ямская ул., д. 86', ARRAY['Филе грудки ц/б охлажденное', 'Фарш из ц/б филейный', 'Крылья ц/б 2ф охлажденные', 'Филе бедра ц/б охлажденное', 'Бедро ц/б охлажденное']::text[]),
	(27, 'Ермолаев', '7203433041', 'Тюменская обл., г. Тюмень, Западносибирская ул., д. 22', ARRAY['Филе грудки ц/б охлажденное', 'Фарш из ц/б филейный', 'Крылья ц/б 2ф охлажденные', 'Бедро ц/б охлажденное', 'Филе бедра ц/б охлажденное']::text[]),
	(28, 'Ермолаев', '7203433041', 'Тюменская обл., г. Тюмень, Николая Гондатти ул., д. 1/3', ARRAY['Филе грудки ц/б охлажденное', 'Фарш из ц/б филейный', 'Крылья ц/б 2ф охлажденные', 'Бедро ц/б охлажденное', 'Филе бедра ц/б охлажденное']::text[]),
	(29, 'Ермолаев', '860902466255', 'Тюменская обл., г. Тюмень, Воронинские горки проезд, д. 178', ARRAY['Филе грудки ц/б охлажденное', 'Крылья ц/б 2ф охлажденные', 'Фарш из ц/б филейный']::text[]),
	(30, 'Ермолаев', '7203265781', 'Тюменская обл., г. Тюмень, Щербакова ул., д. 104', ARRAY['Бедро ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Филе грудки ц/б охлажденное', 'Крылья ц/б 2ф охлажденные', 'Филе бедра ц/б с кожей охлажденное']::text[]),
	(31, 'Ермолаев', '7203265781', 'Российская Федерация, Тюменская обл., г. Тюмень, Тульская ул., д. 7', ARRAY['Бедро ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Филе грудки ц/б охлажденное', 'Крылья ц/б 2ф охлажденные', 'Филе бедра ц/б с кожей охлажденное']::text[]),
	(32, 'Ермолаев', '7203265781', 'Тюменская обл., г. Тюмень, Авторемонтная ул., д. 31а, стр. 12', ARRAY['Бедро ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Филе грудки ц/б охлажденное', 'Крылья ц/б 2ф охлажденные', 'Филе бедра ц/б с кожей охлажденное']::text[]),
	(33, 'Ермолаев', '7203265781', 'Тюменская обл., г. Тюмень, Красных Зорь ул., д. 31, стр. 1', ARRAY['Бедро ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Филе грудки ц/б охлажденное', 'Крылья ц/б 2ф охлажденные', 'Филе бедра ц/б с кожей охлажденное']::text[]),
	(34, 'Ермолаев', '7202238175', 'Российская Федерация, Тюменская обл., г. Тюмень, Беляева ул., д. 35', ARRAY['Фарш из ц/б филейный', 'Крылья ц/б 2ф охлажденные']::text[]),
	(35, 'Ермолаев', '7203580014', 'Тюменская обл., г. Тюмень, Михаила Сперанского ул., д. 42', ARRAY['Филе грудки ц/б замороженное', 'Филе бедра ц/б замороженное', 'Крылья ц/б 2ф замороженные']::text[]),
	(36, 'Столовая', '720312936904', 'Тюменская обл., Тюменский район, с. Кулаково, Ирбитский тракт ул., д. 1', ARRAY['Яйцо куриное С1', 'Желудки ц/б', 'Окорочок ц/б на кости охлажденный', 'Крылья ц/б 3ф охлажденные', 'Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Бедро ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная']::text[]),
	(37, 'Шаверма P13', '450140097524', 'Тюменская обл., Тюменский район, д. Дударева, Сергея Джанбровского ул., д. 27', ARRAY['Филе бедра ц/б охлажденное']::text[]),
	(38, 'Рынок Мальвинка место 19', '722101095244', 'Тюменская обл., г. Тюмень, Бакинских Комиссаров ул., д. 7', ARRAY['Филе грудки ц/б охлажденное', 'Крылья ц/б 3ф охлажденные', 'Каркасы ц/б', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Фарш из ц/б филейный охлажденный', 'Фарш из ц/б филейный замороженный']::text[]),
	(39, 'Рынок Мальвинка место 52А', '722101095244', 'Тюменская обл., г. Тюмень, Бакинских Комиссаров ул., д. 7', ARRAY['Филе грудки ц/б охлажденное', 'Крылья ц/б 3ф охлажденные', 'Каркасы ц/б', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Фарш из ц/б филейный охлажденный', 'Фарш из ц/б филейный замороженный']::text[]),
	(40, 'Кайдзю', '720704187773', 'Тюменская обл., г. Тюмень, Федюнинского ул., д. 55', ARRAY['Филе грудки ц/б охлажденное']::text[]),
	(41, 'Караваево', '7204208585', 'Тюменская обл., г. Тюмень, Валерии Гнаровской ул., д. 5/1', ARRAY['Филе бедра ц/б охлажденное']::text[]),
	(42, 'Караваево', '7204208585', 'Тюменская обл., г. Тюмень, Пермякова ул., д. 48/1', ARRAY['Филе бедра ц/б охлажденное']::text[]),
	(43, 'Караваево', '7204208585', 'Тюменская обл., г. Тюмень, Ямская ул., д. 86/2', ARRAY['Филе бедра ц/б охлажденное']::text[]),
	(44, 'Ковёр', '7203601715', 'Тюменская обл., г. Тюмень, Водопроводная ул., д. 24', ARRAY['Филе бедра ц/б в маринаде охлажденное']::text[]),
	(45, 'Люмян', '7203430964', 'Тюменская обл., г. Тюмень, Луначарского ул., д. 18)', ARRAY['Бедро ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Филе грудки ц/б охлажденное']::text[]),
	(46, 'Майрик', '7203294574', 'Тюменская обл., г. Тюмень, Абатский проезд, д. 2', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Крылья ц/б 2ф охлажденные']::text[]),
	(47, 'Бульвар бар', '7203584643', 'Тюменская обл., г. Тюмень, Герцена ул., д. 63', ARRAY['Тушка Ц/Б 1 сорт охлажденная', 'Яйцо куриное С0 мытое дезинфицированное', 'Филе бедра ц/б охлажденное', 'Филе грудки ц/б охлажденное']::text[]),
	(48, 'МАКС ГРУПП', '7203147604', 'Тюменская обл., г. Тюмень, Олимпийская ул., д. 9', ARRAY['Крылья ц/б 2ф охлажденные', 'Филе бедра ц/б охлажденное', 'Филе грудки ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Окорочок ц/б на кости охлажденный', 'Печень ц/б охлажденная']::text[]),
	(49, 'Кулинарная школа', '7203147604', 'Тюменская обл., г. Тюмень, 25-го Октября ул., д. 34/6', ARRAY['Крылья ц/б 2ф охлажденные', 'Филе бедра ц/б охлажденное', 'Филе грудки ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Окорочок ц/б на кости охлажденный', 'Печень ц/б охлажденная']::text[]),
	(50, 'Столовая', '7202230747', 'Тюменская обл., Тюменский район, с. Перевалово, Тюменский муниципальный район, МО Переваловское, 301 км Федеральной автомобильной дороги "Екатеринбург-Тюмень", строение 17, 1 этаж', ARRAY['Крылья ц/б 3ф охлажденные', 'Грудка ц/б на кости охлажденная', 'Яйцо куриное С1', 'Тушка Ц/Б 1 сорт охлажденная']::text[]),
	(51, 'МАКСИМ-КОФЕЙНИ', '7203227190', 'Тюменская обл., г. Тюмень, Сергея Ильюшина ул., д. 10, к.2', ARRAY['Грудка ц/б на кости замороженная', 'Филе грудки ц/б замороженное', 'Печень ц/б замороженная']::text[]),
	(52, 'МАКСИМ-КОФЕЙНИ', '7203227190', 'Тюменская обл., г. Тюмень, Газовиков ул., д. 73, пом.14', ARRAY['Филе бедра ц/б с кожей охлажденное', 'Крылья ц/б 2ф охлажденные', 'Филе грудки ц/б охлажденное', 'Фарш из ц/б филейный', 'Тушка Ц/Б 1 сорт охлажденная', 'Яйцо куриное С1 мытое дезинфицированное', 'Каркасы ц/б', 'Печень ц/б замороженная']::text[]),
	(53, 'МАКСИМ-КОФЕЙНИ', '7203227190', 'Тюменская обл., г. Тюмень, Олимпийская ул., д. 9', ARRAY['Филе бедра ц/б с кожей охлажденное', 'Крылья ц/б 2ф охлажденные', 'Филе грудки ц/б охлажденное', 'Фарш из ц/б филейный', 'Тушка Ц/Б 1 сорт охлажденная', 'Яйцо куриное С1 мытое дезинфицированное', 'Каркасы ц/б', 'Печень ц/б замороженная']::text[]),
	(54, 'МАКСИМ-КОФЕЙНИ', '7203227190', 'Тюменская обл., г. Тюмень, Республики ул., д. 24', ARRAY['Филе бедра ц/б с кожей охлажденное', 'Крылья ц/б 2ф охлажденные', 'Филе грудки ц/б охлажденное', 'Фарш из ц/б филейный', 'Тушка Ц/Б 1 сорт охлажденная', 'Яйцо куриное С1 мытое дезинфицированное', 'Каркасы ц/б', 'Печень ц/б замороженная']::text[]),
	(55, 'МАКСИМ-КОФЕЙНИ', '7203227190', 'Тюменская обл., г. Тюмень, Михаила Сперанского ул., д. 17/11', ARRAY['Филе бедра ц/б с кожей охлажденное', 'Крылья ц/б 2ф охлажденные', 'Филе грудки ц/б охлажденное', 'Фарш из ц/б филейный', 'Тушка Ц/Б 1 сорт охлажденная', 'Яйцо куриное С1 мытое дезинфицированное', 'Каркасы ц/б', 'Печень ц/б замороженная']::text[]),
	(56, 'МАКСИМ-КОФЕЙНИ', '7203227190', 'Тюменская обл., г. Тюмень, Фабричная ул., д. 9/2', ARRAY['Филе бедра ц/б с кожей охлажденное', 'Крылья ц/б 2ф охлажденные', 'Филе грудки ц/б охлажденное', 'Фарш из ц/б филейный', 'Тушка Ц/Б 1 сорт охлажденная', 'Яйцо куриное С1 мытое дезинфицированное', 'Каркасы ц/б', 'Печень ц/б замороженная']::text[]),
	(57, 'МАКСИМ-КОФЕЙНИ', '7203227190', 'Тюменская обл., г. Тюмень, Герцена ул., д. 94', ARRAY['Филе бедра ц/б с кожей охлажденное', 'Крылья ц/б 2ф охлажденные', 'Филе грудки ц/б охлажденное', 'Фарш из ц/б филейный', 'Тушка Ц/Б 1 сорт охлажденная', 'Яйцо куриное С1 мытое дезинфицированное', 'Каркасы ц/б', 'Печень ц/б замороженная']::text[]),
	(58, 'Посейдон', '7203079746', 'Тюменская обл., г. Тюмень, Ленина ул., д. 2а', ARRAY['Тушка Ц/Б 1 сорт охлажденная']::text[]),
	(59, 'Вангоги', '7203227225', 'Тюменская обл., г. Тюмень, Челюскинцев ул., д. 45', ARRAY['Тушка Ц/Б 1 сорт охлажденная', 'Филе бедра ц/б охлажденное', 'Филе бедра ц/б с кожей охлажденное', 'Филе грудки ц/б с кожей охлажденное', 'Печень ц/б охлажденная', 'Филе грудки ц/б охлажденное']::text[]),
	(60, 'Дача', '7203227225', 'Тюменская обл., Тюменский район, д. Дударева, Тюменская ул., д. 9', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б с кожей охлажденное', 'Филе бедра ц/б охлажденное', 'Печень ц/б охлажденная', 'Желудки ц/б', 'Тушка Ц/Б 1 сорт охлажденная']::text[]),
	(61, 'Максимыч', '7203227225', 'Тюменская обл., г. Тюмень, 50 лет Октября ул., д. 52', ARRAY['Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Филе грудки ц/б охлажденное', 'Крылья ц/б 2ф охлажденные', 'Сердце ц/б', 'Яйцо куриное С0', 'Бедро ц/б охлажденное']::text[]),
	(62, 'Чум', '7203227225', 'Тюменская обл., г. Тюмень, Малыгина ул., д. 59/12', ARRAY['Бедро ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Яйцо куриное С1', 'Филе грудки ц/б охлажденное', 'Печень ц/б охлажденная', 'Голень ц/б охлажденная']::text[]),
	(63, 'Сорренто', '7203565979', 'Тюменская обл., г. Тюмень, Республики ул., д. 142, пом.2', ARRAY['Яйцо куриное С1 мытое дезинфицированное', 'Филе бедра ц/б с кожей охлажденное', 'Филе грудки ц/б с кожей охлажденное', 'Печень ц/б охлажденная', 'Тушка Ц/Б 1 сорт охлажденная']::text[]),
	(64, 'Сыроварня', '7224083006', 'Тюменская обл., г. Тюмень, Советская ул., д. 54', ARRAY['Тушка Ц/Б 1 сорт охлажденная', 'Филе бедра ц/б охлажденное', 'Филе грудки ц/б охлажденное', 'Голень ц/б охлажденная', 'Крылья ц/б 2ф охлажденные']::text[]),
	(65, 'Техникум', '7203575991', 'Тюменская обл., г. Тюмень, Ленина ул., д. 10', ARRAY['Каркасы ц/б', 'Тушка Ц/Б 1 сорт охлажденная', 'Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное']::text[]),
	(66, 'Мангал кебаб', '341911117707', 'Тюменская обл., г. Тюмень, Вадима Бованенко ул., д. 2', ARRAY['Филе грудки ц/б с кожей охлажденное', 'Филе бедра ц/б с кожей охлажденное', 'Крылья ц/б 3ф охлажденные', 'Бедро ц/б охлажденное']::text[]),
	(67, 'Малина', '7203577237', 'Тюменская обл., г. Тюмень, Первомайская ул., д. 18', ARRAY['Филе грудки ц/б охлажденное', 'Каркасы ц/б']::text[]),
	(68, 'МОГУ', '7203520551', 'Тюменская обл., г. Тюмень, Тихий проезд, д. 6', ARRAY['Яйцо куриное С1']::text[]),
	(69, 'Могу кондитерия', '663405149721', 'Тюменская обл., г. Тюмень, Тимофея Кармацкого ул., д. 11, стр. корп.2, пом.1', ARRAY['Яйцо куриное С1']::text[]),
	(70, 'Плов', '7203584650', 'Тюменская обл., г. Тюмень, Советская ул., д. 21', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Каркасы ц/б', 'Яйцо куриное С0 мытое дезинфицированное']::text[]),
	(71, 'Плов', '7203583978', 'Тюменская обл., г. Тюмень, Газовиков ул., д. 61, пом.1', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Каркасы ц/б', 'Яйцо куриное С0 мытое дезинфицированное']::text[]),
	(72, 'Плов', '7203583978', 'Тюменская обл., г. Тюмень, Республики ул., д. 131', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Каркасы ц/б', 'Яйцо куриное С0 мытое дезинфицированное']::text[]),
	(73, 'Плов', '7203583985', 'Тюменская обл., г. Тюмень, Ленина ул., д. 58В', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Каркасы ц/б', 'Яйцо куриное С0 мытое дезинфицированное']::text[]),
	(74, 'Плов', '7203583985', 'Тюменская обл., г. Тюмень, Обдорская ул., д. 1, стр. корп.2', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Тушка Ц/Б 1 сорт охлажденная', 'Каркасы ц/б', 'Яйцо куриное С0 мытое дезинфицированное']::text[]),
	(75, 'Пицца дей', '451002778753', 'Тюменская обл., г. Тюмень, Александра Протозанова ул., д. 8 к. 1, пом.4', ARRAY['Филе бедра ц/б с кожей охлажденное', 'Филе грудки ц/б с кожей охлажденное']::text[]),
	(76, 'ТУТУ', '860605215146', 'Тюменская обл., г. Тюмень, Тимофея Чаркова ул., д. 60', ARRAY['Филе грудки ц/б охлажденное', 'Филе бедра ц/б охлажденное', 'Фарш из ц/б филейный замороженный', 'Бедро ц/б охлажденное']::text[]),
	(77, 'Хот донер', '615424905887', 'Тюменская обл., г. Тюмень, Валении Гнаровской ул., д. 12', ARRAY['Филе грудки ц/б охлажденное', 'Яйцо куриное С1', 'Филе ц/б с кожей в маринаде охлажденное', 'Крылья ц/б 2ф охлажденные', 'Голень ц/б охлажденная', 'Филе бедра ц/б охлажденное']::text[]),
	(78, 'Хот донер', '722201390204', 'Тюменская обл., г. Тюмень, Льва Толстого ул., д. 47', ARRAY['Филе грудки ц/б охлажденное', 'Яйцо куриное С1', 'Филе ц/б с кожей в маринаде охлажденное', 'Крылья ц/б 2ф охлажденные', 'Голень ц/б охлажденная', 'Филе бедра ц/б охлажденное']::text[]),
	(79, 'Хот донер', '722201390204', 'Тюменская область, г Тюмень, Тимуровцев ул., д. 2в', ARRAY['Филе грудки ц/б охлажденное', 'Яйцо куриное С1', 'Филе ц/б с кожей в маринаде охлажденное', 'Крылья ц/б 2ф охлажденные', 'Голень ц/б охлажденная', 'Филе бедра ц/б охлажденное']::text[]),
	(80, 'Шашлыкофф', '7203604843', 'Тюменская обл., г. Тюмень, Пермякова ул., д. 62 к. 1', ARRAY['Крылья ц/б 2ф замороженные', 'Филе бедра ц/б с кожей замороженное', 'Филе грудки ц/б замороженное', 'Голень ц/б замороженная', 'Печень ц/б замороженная']::text[]),
	(81, 'Шашлыкофф', '7203604843', 'Тюменская обл., г. Тюмень, Ленина ул., д. 54в', ARRAY['Крылья ц/б 2ф замороженные', 'Филе бедра ц/б с кожей замороженное', 'Филе грудки ц/б замороженное', 'Голень ц/б замороженная', 'Печень ц/б замороженная']::text[]),
	(82, 'Бургерная кампус', '7224098490', 'Тюменская обл., г. Тюмень, Республики ул., д. 8а', ARRAY['Крылья ц/б 3ф охлажденные', 'Филе бедра ц/б охлажденное', 'Филе грудки ц/б охлажденное']::text[]),
	(83, 'Сочень', '720201041016', 'Тюменская обл., г. Тюмень, Энергетиков ул., д. 50, помещ. 2', ARRAY['Филе бедра ц/б охлажденное', 'Филе грудки ц/б охлажденное', 'Яйцо куриное С1']::text[]),
	(84, 'Чина', '7204170370', 'Тюменская обл., г. Тюмень, Комсомольская ул., д. 8', ARRAY['Крылья ц/б 2ф замороженные', 'Филе бедра ц/б охлажденное', 'Яйцо куриное С1']::text[]),
	(85, 'Море вкуса', '7203524789', 'Тюменская обл., г. Тюмень, 30 лет Победы ул., д. 25', ARRAY['фарш из ц/б филейный', 'Тушка Ц/Б 1 сорт охлажденная', 'Филе бедра ц/б с кожей охлажденное', 'Крылья ц/б 2ф замороженные', 'Окорочок ц/б на кости охлажденный']::text[]);

CREATE TEMPORARY TABLE migration_order_history_product_alias (
	source_name_key text PRIMARY KEY,
	canonical_product_name text NOT NULL
) ON COMMIT DROP;

INSERT INTO migration_order_history_product_alias (
	source_name_key,
	canonical_product_name
)
VALUES
	('бедро ц/б охлажденное', 'Бедро цыпленка-бройлера охлажденное'),
	('голень ц/б замороженная', 'Голень цыпленка-бройлера замороженная'),
	('голень ц/б охлажденная', 'Голень цыпленка-бройлера охлажденная'),
	('грудка ц/б на кости замороженная', 'Грудка цыпленка-бройлера на кости замороженная'),
	('грудка ц/б на кости охлажденная', 'Грудка цыпленка-бройлера на кости охлажденная'),
	('желудки ц/б', 'Желудки цыпленка-бройлера охлажденные'),
	('каркасы ц/б', 'Суповой набор из цыпленка-бройлера (каркасы ц/б)'),
	('крылья ц/б 2ф замороженные', 'Крылья цыпленка-бройлера замороженные 2ф'),
	('крылья ц/б 2ф охлажденные', 'Крылья цыпленка-бройлера охлажденные 2ф'),
	('крылья ц/б 3ф охлажденные', 'Крылья цыпленка-бройлера охлажденные 3ф'),
	('окорочок ц/б на кости охлажденный', 'Окорочок цыпленка-бройлера на кости охлажденный'),
	('печень ц/б замороженная', 'Печень цыпленка-бройлера замороженная'),
	('печень ц/б охлажденная', 'Печень цыпленка-бройлера охлажденная'),
	('сердце ц/б', 'Сердце цыпленка-бройлера охлажденное'),
	('тушка ц/б 1 сорт охлажденная', 'Тушка цыпленка-бройлера 1-го сорта охлажденная'),
	('фарш из ц/б филейный', 'Фарш куриный филейный охлажденный'),
	('фарш из ц/б филейный замороженный', 'Фарш куриный филейный замороженный'),
	('фарш из ц/б филейный охлажденный', 'Фарш куриный филейный охлажденный'),
	('филе бедра ц/б в маринаде охлажденное', 'Филе бедра цыпленка-бройлера в маринаде охлажденное'),
	('филе бедра ц/б замороженное', 'Филе бедра цыпленка-бройлера замороженое'),
	('филе бедра ц/б охлажденное', 'Филе бедра цыпленка-бройлера охлажденное'),
	('филе бедра ц/б с кожей замороженное', 'Филе бедра цыпленка-бройлера с кожей замороженное'),
	('филе бедра ц/б с кожей охлажденное', 'Филе бедра цыпленка-бройлера с кожей охлажденное'),
	('филе грудки ц/б замороженное', 'Филе грудки цыпленка-бройлера замороженное'),
	('филе грудки ц/б охлажденное', 'Филе грудки цыпленка-бройлера охлажденное'),
	('филе грудки ц/б с кожей охлажденное', 'Филе грудки цыпленка-бройлера с кожей охлажденное'),
	('филе ц/б с кожей в маринаде охлажденное', 'Филе цыпленка-бройлера с кожей в маринаде охлажденное'),
	('яйцо куриное с0', 'Яйцо куриное пищевое С0'),
	('яйцо куриное с0 мытое дезинфицированное', 'Яйцо куриное пищевое С0 мытое дезинфицированное'),
	('яйцо куриное с1', 'Яйцо куриное пищевое С1'),
	('яйцо куриное с1 мытое дезинфицированное', 'Яйцо куриное пищевое С1 мытое дезинфицированное');

CREATE TEMPORARY TABLE migration_order_history_source_items (
	source_row integer NOT NULL,
	line_num integer NOT NULL,
	source_name_key text NOT NULL,
	PRIMARY KEY (source_row, line_num)
) ON COMMIT DROP;

INSERT INTO migration_order_history_source_items (
	source_row,
	line_num,
	source_name_key
)
SELECT
	source.source_row,
	item.ordinality::integer,
	lower(btrim(item.product_name))
FROM migration_order_history_source AS source
CROSS JOIN LATERAL unnest(source.product_names)
	WITH ORDINALITY AS item(product_name, ordinality);

DO $migration$
DECLARE
	v_source_count integer;
	v_item_count integer;
	v_problem text;
	v_marker_prefix constant text := '[migration:000051_customer_order_history_from_excel;source_row=';
BEGIN
	SELECT count(*) INTO v_source_count
	FROM migration_order_history_source;

	SELECT count(*) INTO v_item_count
	FROM migration_order_history_source_items;

	IF v_source_count <> 84 THEN
		RAISE EXCEPTION 'Expected 84 history source rows, found %', v_source_count;
	END IF;

	IF v_item_count <> 379 THEN
		RAISE EXCEPTION 'Expected 379 expanded history items, found %', v_item_count;
	END IF;

	SELECT string_agg(
		format('row %s: %s', duplicate.source_row, duplicate.source_name_key),
		'; ' ORDER BY duplicate.source_row, duplicate.source_name_key
	)
	INTO v_problem
	FROM (
		SELECT source_row, source_name_key
		FROM migration_order_history_source_items
		GROUP BY source_row, source_name_key
		HAVING count(*) > 1
	) AS duplicate;

	IF v_problem IS NOT NULL THEN
		RAISE EXCEPTION 'Duplicate products in history source rows: %', v_problem;
	END IF;

	IF EXISTS (
		SELECT 1
		FROM public.orders
		WHERE position(v_marker_prefix IN coalesce(comment_admin, '')) = 1
	) THEN
		RAISE EXCEPTION 'Customer order history migration marker already exists';
	END IF;
END
$migration$;

-- Resolve every customer exactly once by INN before inserting any documents.
DO $migration$
DECLARE
	v_problem text;
BEGIN
	SELECT string_agg(
		format('%s (%s matches)', problem.inn, problem.match_count),
		'; ' ORDER BY problem.inn
	)
	INTO v_problem
	FROM (
		SELECT source.inn, count(customer.id) AS match_count
		FROM (
			SELECT DISTINCT inn
			FROM migration_order_history_source
		) AS source
		LEFT JOIN public.customers AS customer
			ON customer.inn = source.inn
		GROUP BY source.inn
		HAVING count(customer.id) <> 1
	) AS problem;

	IF v_problem IS NOT NULL THEN
		RAISE EXCEPTION 'Customer lookup by INN must resolve exactly once: %', v_problem;
	END IF;
END
$migration$;

-- Validate the exact final state instead of selecting an arbitrary final status.
DO $migration$
DECLARE
	v_status_count integer;
BEGIN
	SELECT count(*) INTO v_status_count
	FROM public.order_statuses
	WHERE code = 'completed';

	IF v_status_count <> 1 THEN
		RAISE EXCEPTION 'Order status code completed must resolve exactly once; found %', v_status_count;
	END IF;
END
$migration$;

-- First match by INN + address. The source has two market stalls at one exact
-- address; only in that case the workbook's Заведение label disambiguates them.
CREATE TEMPORARY TABLE migration_order_history_sale_place_candidates
ON COMMIT DROP
AS
WITH address_candidates AS (
	SELECT
		source.source_row,
		customer.id AS customer_id,
		sale_place.id AS sale_place_id,
		count(sale_place.id) OVER (
			PARTITION BY source.source_row
		) AS address_match_count,
		lower(btrim(sale_place.name)) = lower(btrim(source.place_name)) AS name_matches
	FROM migration_order_history_source AS source
	JOIN public.customers AS customer
		ON customer.inn = source.inn
	LEFT JOIN public.customer_sale_places AS sale_place
		ON sale_place.customer_id = customer.id
		AND lower(btrim(coalesce(sale_place.address, ''))) = lower(btrim(source.address))
)
SELECT
	source_row,
	customer_id,
	sale_place_id
FROM address_candidates
WHERE sale_place_id IS NOT NULL
	AND (address_match_count = 1 OR name_matches);

DO $migration$
DECLARE
	v_problem text;
	v_resolved_count integer;
	v_distinct_sale_place_count integer;
BEGIN
	SELECT string_agg(
		format(
			'row %s, INN %s, address %s (%s matches)',
			problem.source_row,
			problem.inn,
			problem.address,
			problem.match_count
		),
		'; ' ORDER BY problem.source_row
	)
	INTO v_problem
	FROM (
		SELECT
			source.source_row,
			source.inn,
			source.address,
			count(candidate.sale_place_id) AS match_count
		FROM migration_order_history_source AS source
		LEFT JOIN migration_order_history_sale_place_candidates AS candidate
			ON candidate.source_row = source.source_row
		GROUP BY source.source_row, source.inn, source.address
		HAVING count(candidate.sale_place_id) <> 1
	) AS problem;

	IF v_problem IS NOT NULL THEN
		RAISE EXCEPTION 'Sale-place lookup must resolve exactly once: %', v_problem;
	END IF;

	SELECT count(*), count(DISTINCT sale_place_id)
	INTO v_resolved_count, v_distinct_sale_place_count
	FROM migration_order_history_sale_place_candidates;

	IF v_resolved_count <> 84 OR v_distinct_sale_place_count <> 84 THEN
		RAISE EXCEPTION
			'Expected 84 distinct resolved sale places; found % rows and % distinct places',
			v_resolved_count,
			v_distinct_sale_place_count;
	END IF;
END
$migration$;

-- Resolve all abbreviated product names before creating any orders.
CREATE TEMPORARY TABLE migration_order_history_product_resolution
ON COMMIT DROP
AS
WITH required_product AS (
	SELECT DISTINCT source_name_key
	FROM migration_order_history_source_items
)
SELECT
	required_product.source_name_key,
	alias.canonical_product_name,
	min(product.id)::integer AS product_id,
	min(product.measure_unit_id)::integer AS measure_unit_id,
	count(product.id)::integer AS product_match_count,
	count(product.measure_unit_id)::integer AS product_with_unit_count
FROM required_product
LEFT JOIN migration_order_history_product_alias AS alias
	ON alias.source_name_key = required_product.source_name_key
LEFT JOIN public.products AS product
	ON product.is_group = false
	AND product.is_active = true
	AND lower(btrim(product.name)) = lower(btrim(alias.canonical_product_name))
GROUP BY required_product.source_name_key, alias.canonical_product_name;

DO $migration$
DECLARE
	v_problem text;
	v_required_count integer;
BEGIN
	SELECT count(*) INTO v_required_count
	FROM migration_order_history_product_resolution;

	IF v_required_count <> 31 THEN
		RAISE EXCEPTION 'Expected 31 normalized source product names, found %', v_required_count;
	END IF;

	SELECT string_agg(
		format(
			'%s -> %s (%s product matches, %s with unit)',
			resolution.source_name_key,
			coalesce(resolution.canonical_product_name, '<no alias>'),
			resolution.product_match_count,
			resolution.product_with_unit_count
		),
		'; ' ORDER BY resolution.source_name_key
	)
	INTO v_problem
	FROM migration_order_history_product_resolution AS resolution
	WHERE resolution.canonical_product_name IS NULL
		OR resolution.product_match_count <> 1
		OR resolution.product_with_unit_count <> 1;

	IF v_problem IS NOT NULL THEN
		RAISE EXCEPTION 'Product lookup must resolve to one active non-group product with a measure unit: %', v_problem;
	END IF;

	SELECT string_agg(
		format('row %s -> product id %s', duplicate.source_row, duplicate.product_id),
		'; ' ORDER BY duplicate.source_row, duplicate.product_id
	)
	INTO v_problem
	FROM (
		SELECT
			item.source_row,
			resolution.product_id,
			count(*) AS match_count
		FROM migration_order_history_source_items AS item
		JOIN migration_order_history_product_resolution AS resolution
			ON resolution.source_name_key = item.source_name_key
		GROUP BY item.source_row, resolution.product_id
		HAVING count(*) > 1
	) AS duplicate;

	IF v_problem IS NOT NULL THEN
		RAISE EXCEPTION 'Product aliases collapse multiple lines in one history order: %', v_problem;
	END IF;
END
$migration$;

CREATE TEMPORARY TABLE migration_order_history_inserted_orders (
	source_row integer PRIMARY KEY,
	customer_id integer NOT NULL,
	sale_place_id integer NOT NULL,
	order_id integer UNIQUE
) ON COMMIT DROP;

INSERT INTO migration_order_history_inserted_orders (
	source_row,
	customer_id,
	sale_place_id
)
SELECT
	source_row,
	customer_id,
	sale_place_id
FROM migration_order_history_sale_place_candidates
ORDER BY source_row;

DO $migration$
DECLARE
	v_source record;
	v_order_id integer;
	v_status_id integer;
BEGIN
	SELECT id INTO v_status_id
	FROM public.order_statuses
	WHERE code = 'completed';

	FOR v_source IN
		SELECT source_row, customer_id, sale_place_id
		FROM migration_order_history_inserted_orders
		ORDER BY source_row
	LOOP
		INSERT INTO public.orders (
			for_date,
			customer_id,
			customer_sale_place_id,
			status_id,
			comment_admin
		)
		VALUES (
			DATE '2026-09-15',
			v_source.customer_id,
			v_source.sale_place_id,
			v_status_id,
			format(
				'[migration:000051_customer_order_history_from_excel;source_row=%s] История заказов из файла Клиенты+что покупают.xlsx',
				v_source.source_row
			)
		)
		RETURNING id INTO v_order_id;

		UPDATE migration_order_history_inserted_orders
		SET order_id = v_order_id
		WHERE source_row = v_source.source_row;
	END LOOP;
END
$migration$;

INSERT INTO public.order_items (
	line_num,
	order_id,
	product_id,
	measure_unit_id,
	quant_required,
	quant
)
SELECT
	item.line_num,
	inserted_order.order_id,
	resolution.product_id,
	resolution.measure_unit_id,
	1,
	1
FROM migration_order_history_source_items AS item
JOIN migration_order_history_product_resolution AS resolution
	ON resolution.source_name_key = item.source_name_key
JOIN migration_order_history_inserted_orders AS inserted_order
	ON inserted_order.source_row = item.source_row
ORDER BY item.source_row, item.line_num;

-- Synthetic quantity 1 exists only to satisfy Order item requirements. Do not
-- post these documents to ra_products: MAX history reads orders/order_items
-- directly, while posting would pollute operational product totals.
DO $migration$
DECLARE
	v_order_count integer;
	v_item_count integer;
BEGIN
	SELECT count(*) INTO v_order_count
	FROM migration_order_history_inserted_orders
	WHERE order_id IS NOT NULL;

	SELECT count(*) INTO v_item_count
	FROM public.order_items AS item
	JOIN migration_order_history_inserted_orders AS inserted_order
		ON inserted_order.order_id = item.order_id;

	IF v_order_count <> 84 OR v_item_count <> 379 THEN
		RAISE EXCEPTION
			'History import postcondition failed: expected 84 orders and 379 items; found % and %',
			v_order_count,
			v_item_count;
	END IF;
END
$migration$;

COMMIT;
