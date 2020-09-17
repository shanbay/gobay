module github.com/shanbay/gobay

go 1.13

require (
	github.com/RichardKnop/logging v0.0.0-20190827224416-1a693bdd4fae
	github.com/RichardKnop/machinery v1.9.2
	github.com/facebook/ent v0.4.0
	github.com/getsentry/sentry-go v0.7.0
	github.com/go-openapi/runtime v0.19.22
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/go-sql-driver/mysql v1.5.1-0.20200311113236-681ffa848bae
	github.com/golang/protobuf v1.4.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.1
	github.com/iancoleman/strcase v0.0.0-20191112232945-16388991a334
	github.com/markbates/pkger v0.17.1
	github.com/mattn/go-sqlite3 v1.14.2
	github.com/mitchellh/mapstructure v1.3.3
	github.com/satori/go.uuid v1.2.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.1
	github.com/streadway/amqp v1.0.0
	github.com/stretchr/testify v1.6.1
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	go.elastic.co/apm v1.8.0
	go.elastic.co/apm/module/apmgoredis v1.8.0
	go.elastic.co/apm/module/apmgrpc v1.8.0
	go.elastic.co/apm/module/apmsql v1.8.0
	google.golang.org/grpc v1.31.1
	google.golang.org/grpc/examples v0.0.0-20200709004140-9af290fac4b2 // indirect
)

replace github.com/facebook/ent => github.com/shanbay/ent v0.4.0
