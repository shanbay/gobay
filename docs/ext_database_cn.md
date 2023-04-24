# 接入数据库

## config 配置

打开 config.yaml 文件，这几个配置

```yaml
  db_driver: mysql
  db_url: 'root:dbpassword@(mysql:3306)/helloworld?charset=utf8mb4&parseTime=true&loc=UTC&interpolateParams=true'
```

## 加载数据库用的 ent extension

- `app/extensions.go`

```go
package app

import (
  schema "git.17bdc.com/backend/helloworld/gen/entschema"
  "entgo.io/ent/dialect"
  _ "github.com/go-sql-driver/mysql"
  "github.com/shanbay/gobay"
  "github.com/shanbay/gobay/extensions/entext"
  _ "go.elastic.co/apm/module/apmsql/mysql"
)

func Extensions() map[gobay.Key]gobay.Extension {
  return map[gobay.Key]gobay.Extension{
    // ...
    "entext": &entext.EntExt{
      NS: "db_",
      NewClient: func(opt interface{}) entext.Client {
        return schema.NewClient(opt.(schema.Option))
      },
      Driver: func(drv dialect.Driver) interface{} {
        return schema.Driver(drv)
      },
    },
    // ...
  }
}

var (
  // ...
  EntClient      *schema.Client
  // ...
)

func InitExts(app *gobay.Application) {
  // ...
  EntClient = app.Get("entext").Object().(*schema.Client)
  // ...
}
```

## 配置数据库 table schema

在 `spec/schema/` 里，创建新的 schema 文件，`spec/schema/sample.go`

在里面配置好新model的字段设置(fields)，索引设置(indexes), table名称(Config->table)。

```go
package schema

import (
  "time"

  "entgo.io/ent"
  "entgo.io/ent/dialect"
  "entgo.io/ent/schema/field"
  "entgo.io/ent/schema/index"
)

// Sample holds the schema definition for the Sample entity.
type Sample struct {
  ent.Schema
}

// Fields of the Sample.
func (Sample) Fields() []ent.Field {
  return []ent.Field{
    field.Uint64("id").Unique().Immutable(),
    field.String("name").Immutable(),
    field.Time("created_at").Default(time.Now).SchemaType(map[string]string{
      dialect.MySQL: "datetime",
    }),
    field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{
      dialect.MySQL: "datetime",
    }),
  }
}

// Config of the Sample.
func (Sample) Config() ent.Config {
  return ent.Config{
    Table: "sample",
  }
}

func (Sample) Indexes() []ent.Index {
  return []ent.Index{
    index.Fields("name", "created_at"),
  }
}
```

## 生成 ent 用的代码文件

```sh
make entgen
```

这会生成 schema 文件（`helloworld/gen/entschema`），和每个 model 的文件（`helloworld/gen/entschema/sample`）

## 使用 ent client 来请求数据库

在 `app/models` 文件夹内，创建新 model 的文件， `app/models/sample.go`

在里面写入 get/create/update/delete 等 function 。

```go
package models

import (
  "context"
  "helloworld/app"
  schema "helloworld/gen/entschema"
  sSample "helloworld/gen/entschema/sample"
)

func SampleCreate(ctx context.Context, name string) (*schema.Sample, error) {
  Sample, err := app.EntClient.Sample.Create().
    SetName(name).
    Save(ctx)
  if err != nil {
    return nil, err
  }
  return Sample, nil
}

func SampleUpdateStatus(
  ctx context.Context,
  id uint64,
  name string,
) (*schema.Sample, error) {
  Sample, err := app.EntClient.Sample.UpdateOneID(id).
    SetName(name).
    Save(ctx)
  if err != nil {
    return nil, err
  }
  return Sample, nil
}

func SampleGetLastByName(ctx context.Context, name string) (*schema.Sample, error) {
  Samples, err := app.EntClient.Sample.Query().
    Where(sSample.Name(name)).
    Order(schema.Desc(sSample.FieldCreatedAt), schema.Desc(sSample.FieldID)).
    Limit(1).
    All(ctx)
  if err != nil {
    if schema.IsNotFound(err) {
      return nil, nil
    }
    return nil, err
  }
  if len(Samples) == 0 {
    return nil, nil
  }
  return Samples[0], nil
}
```

## 业务逻辑里调用

业务逻辑里，可以使用 `models.SampleGetLastByName(ctx, name)` 这样的方法调用和修改数据库数据。
