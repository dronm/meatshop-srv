pg_dump \
	-h localhost \
	-U meatshop \
	--format=plain \
	--no-owner \
	--no-acl \
	--quote-all-identifiers \
	-d meatshop \
	-f meatshop.sql

grep -nE '^\\(un)?restrict ' meatshop.sql
sed '/^\\restrict /d; /^\\unrestrict /d' meatshop.dump > meatshop.sql
sed -i.bak \
	-e '/^\\restrict /d' \
	-e '/^\\unrestrict /d' \
	-e '/^SET transaction_timeout = /d' \
	meatshop.sql

psql \
	-U meatshop \
	-d meatshop \
	-X \
	-v ON_ERROR_STOP=1 \
	-f meatshop.sql

pg_restore \
	-h localhost \
	-U postgres \
	-d meatshop \
	--no-owner \
	--no-acl \
	--role=meatshop \
	meatshop.dump
