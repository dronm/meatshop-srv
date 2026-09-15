module github.com/dronm/meatshop

go 1.25.0

replace github.com/dronm/webapp => /home/andrey/go/webapp

replace github.com/dronm/ds/v4 => /home/andrey/go/ds/v4

replace github.com/dronm/session => /home/andrey/go/session

replace github.com/dronm/modelbind => /home/andrey/go/modelbind

replace github.com/dronm/codegen => /home/andrey/go/codegen

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dronm/codegen v0.1.0 // indirect
	github.com/dronm/ds/v4 v4.0.0-20260612024620-1d1425e7a834 // indirect
	github.com/dronm/modelbind v0.0.0-20260504034703-7290b55279cc // indirect
	github.com/dronm/session v0.0.0-20251220011348-444cd0bd6dad // indirect
	github.com/dronm/webapp v0.0.0-20260710081134-deaacd7d2b42 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.9.2 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/mssola/user_agent v0.6.0 // indirect
	github.com/redis/go-redis/v9 v9.17.2 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

tool github.com/dronm/codegen/cmd/codegen
