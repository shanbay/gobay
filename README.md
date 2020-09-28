# gobay
[![Go Report Card](https://goreportcard.com/badge/github.com/shanbay/gobay)](https://goreportcard.com/report/github.com/shanbay/gobay)
[![Build Status](https://travis-ci.org/shanbay/gobay.svg?branch=master)](https://travis-ci.org/shanbay/gobay)
[![](https://img.shields.io/:license-mit-blue.svg?style=flat-square)](https://shanbay.mit-license.org)

# 脚手架

```bash
# go get github.com/shanbay/gobay/cmd/gobay
gobay new github.com/me/mine_project
```

[详细说明](cmd/gobay/README.md)

# 贡献代码

## 如何为gobay编写extension

1. 实现`gobay.Extension`
2. 每一个ext目录都是你的例子

# ext

### ent/orm

**关于时间**

```
// 如果需要开启 parseTime，在 dsn 中加上参数：
// 显式指定loc为UTC
parseTime=True&loc=UTC
```

> 虽然 `loc` 默认值为 `UTC`，但是这依赖于默认情况。默认情况有可能发生改变。所以我们推荐显式指定 `loc`
