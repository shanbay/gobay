# 新建扩展

当你想接入新的sdk等工具时，而gobay里找不到这个工具，可以考虑新建一个自己的扩展。跟DB/Redis等一样使用。

这儿我们建立一个 elasticsearch v7 用的扩展。

## 调研SDK

简单搜索过后，对于elasticsearch，有两个常用的golang用的sdk：

- 官方的 https://github.com/elastic/go-elasticsearch
- 非官方但也很好用的 https://github.com/olivere/elastic

你可以调研一下哪个更适合你和你的项目。这里我们不讨论哪个更好，我就用官方的go-elasticsearch了。

## 准备扩展组件 extension

- 在 `app` 文件夹下，创建 `app/extensions/elasticsearchv7` 文件夹，再再里面创建 `app/extensions/elasticsearchv7/ext.go`

```go
package elasticsearchv7ext

import (
  "errors"
  "log"

  "github.com/elastic/go-elasticsearch/v7"
  "github.com/shanbay/gobay"
)

type ElasticSearchV7Ext struct {
  app    *gobay.Application
  NS     string
  client *elasticsearch.Client
  Config *elasticsearch.Config
}

var _ gobay.Extension = (*ElasticSearchV7Ext)(nil)

// Init -
func (e *ElasticSearchV7Ext) Init(app *gobay.Application) error {
  var err error
  if e.NS == "" {
    return errors.New("lack of NS")
  }
  e.app = app
  // 从 config 中，按 prefix 为 `"ElasticSearchV7Ext{NS: "elasticsearch_"},` 的 NS 来寻找config内容。
  config := gobay.GetConfigByPrefix(app.Config(), e.NS, true)
  if err = config.Unmarshal(&e.Config); err != nil {
    return err
  }
  e.client, err = elasticsearch.NewClient(*e.Config)
  if err != nil {
    log.Fatalf("Error creating elastic search client: %s", err)
  }
  return nil
}

// Object return lru cache client
func (e *ElasticSearchV7Ext) Object() interface{} {
  return e.client
}

func (e *ElasticSearchV7Ext) Client() *elasticsearch.Client {
  return e.client
}

// Close close client
func (e *ElasticSearchV7Ext) Close() error {
  return nil
}

// Application
func (e *ElasticSearchV7Ext) Application() *gobay.Application {
  return e.app
}
```

与其他 extension 保持相同的 API， 有 `Init(), Object(), Client(), Close(), Application()` 函数，即使里面可能只是个简单的 `return nil`。

## 使用 extension

像其他扩展一样，配置 `config.yaml`，在 `app/extensions.go` 里配置 extension 启动的代码，并在逻辑中调用即可。

- `config.yaml`

```yaml
  elasticsearch_addresses:
    - "http://elasticsearch:9200"
  elasticsearch_username: username
  elasticsearch_password: password
```

- `app/extensions.go`

```go
package app

import (
  "helloworld/app/ext/elasticsearchv7ext"
)

func Extensions() map[gobay.Key]gobay.Extension {
  return map[gobay.Key]gobay.Extension{
    // ...
    "esClient":  &elasticsearchv7ext.ElasticSearchV7Ext{NS: "elasticsearch_"},
    // ...
  }
}

var (
  // ...
  ESClient       *elasticsearchv7.Client
  // ...
)

func InitExts(app *gobay.Application) {
  // ...
  ESClient = app.Get("esClient").Object().(*elasticsearchv7.Client)
  // ...
}
```

- 项目逻辑代码（elasticsearch查询）

```go
  // ...
  res, err := app.ESClient.Search(
    app.ESClient.Search.WithContext(context.Background()),
    app.ESClient.Search.WithIndex("index-name"),
    app.ESClient.Search.WithSort("@timestamp:asc"),
    app.ESClient.Search.WithBody(&buf), // query json string
    app.ESClient.Search.WithTimeout(480*time.Millisecond),
    app.ESClient.Search.WithSize(1000),
  )
  if err != nil {
    return nil, err
  }
  defer res.Body.Close()
  // ...
```
