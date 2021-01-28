module github.com/shanbay/gobay

go 1.15

require (
	github.com/RichardKnop/logging v0.0.0-20190827224416-1a693bdd4fae
	github.com/RichardKnop/machinery v1.10.0
	github.com/facebook/ent v0.4.0
	github.com/getsentry/sentry-go v0.9.0
	github.com/go-openapi/runtime v0.19.24
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/go-redis/redis/v8 v8.3.4
	github.com/go-redis/redis_rate/v9 v9.1.1
	github.com/go-sql-driver/mysql v1.5.1-0.20200311113236-681ffa848bae
	github.com/golang/protobuf v1.4.3
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/hashicorp/go-multierror v1.1.0
	github.com/iancoleman/strcase v0.1.2
	github.com/markbates/pkger v0.17.1
	github.com/mattn/go-sqlite3 v1.14.6
	github.com/mitchellh/mapstructure v1.4.1
	github.com/pkg/errors v0.9.1
	github.com/satori/go.uuid v1.2.0
	github.com/spf13/cobra v1.1.1
	github.com/spf13/viper v1.7.1
	github.com/streadway/amqp v1.0.0
	github.com/stretchr/objx v0.3.0
	github.com/stretchr/testify v1.7.0
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	go.elastic.co/apm v1.9.0
	go.elastic.co/apm/module/apmgoredis v1.9.0
	go.elastic.co/apm/module/apmgrpc v1.9.0
	go.elastic.co/apm/module/apmsql v1.9.0
	google.golang.org/grpc v1.35.0
	google.golang.org/grpc/examples v0.0.0-20200925170654-e6c98a478e62 // indirect
)

replace (
	github.com/RichardKnop/machinery => github.com/RichardKnop/machinery v1.9.7
	github.com/facebook/ent => github.com/shanbay/ent v0.4.0
)
