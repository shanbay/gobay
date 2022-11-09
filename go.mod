module github.com/shanbay/gobay

go 1.15

require (
	github.com/RichardKnop/logging v0.0.0-20190827224416-1a693bdd4fae
	github.com/RichardKnop/machinery v1.10.6
	github.com/elastic/go-sysinfo v1.7.0 // indirect
	github.com/elastic/go-windows v1.0.1 // indirect
	github.com/facebook/ent v0.4.0
	github.com/getsentry/sentry-go v0.15.0
	github.com/go-co-op/gocron v1.17.1
	github.com/go-openapi/runtime v0.23.2
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/go-sql-driver/mysql v1.5.1-0.20200311113236-681ffa848bae
	github.com/golang/mock v1.5.0
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.3.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/hashicorp/go-multierror v1.1.0
	github.com/iancoleman/strcase v0.1.3
	github.com/labstack/echo/v4 v4.9.0
	github.com/markbates/pkger v0.17.1
	github.com/mattn/go-sqlite3 v1.14.6
	github.com/mitchellh/mapstructure v1.4.3
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.12.1
	github.com/satori/go.uuid v1.2.0
	github.com/spf13/cobra v1.4.0
	github.com/spf13/viper v1.8.1
	github.com/streadway/amqp v1.0.0
	github.com/stretchr/objx v0.4.0
	github.com/stretchr/testify v1.8.0
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	go.elastic.co/apm v1.13.0
	go.elastic.co/apm/module/apmgoredis v1.9.0
	go.elastic.co/apm/module/apmgrpc v1.9.0
	go.elastic.co/apm/module/apmsql v1.9.0
	golang.org/x/sync v0.1.0 // indirect
	google.golang.org/grpc v1.38.0
	google.golang.org/grpc/examples v0.0.0-20200925170654-e6c98a478e62 // indirect
	howett.net/plist v0.0.0-20201203080718-1454fab16a06 // indirect
)

replace (
	github.com/RichardKnop/machinery => github.com/shanbay/machinery v1.10.7-0.20211122094032-89c5c12d758a
	github.com/facebook/ent => github.com/shanbay/ent v0.4.0
	github.com/streadway/amqp => github.com/shanbay/amqp v1.0.1-0.20210728052407-b63250c049f2
)
