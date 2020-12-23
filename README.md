# [gobay](https://shanbay.github.io/gobay)
[![Go Report Card](https://goreportcard.com/badge/github.com/shanbay/gobay)](https://goreportcard.com/report/github.com/shanbay/gobay)
[![Build Status](https://github.com/shanbay/gobay/workflows/CI/badge.svg)](https://github.com/shanbay/gobay/actions)
[![](https://img.shields.io/:license-mit-blue.svg?style=flat-square)](https://shanbay.mit-license.org)

# Documentation

- [Official documentation](https://shanbay.github.io/gobay)
  - [Changelog](https://shanbay.github.io/gobay/#/CHANGELOG)
  - [Installation](https://shanbay.github.io/gobay/#/installation)
  - [Quick Start](https://shanbay.github.io/gobay/#/quickstart)
  - [Project Structure](https://shanbay.github.io/gobay/#/structure)

# Contributing

## 如何为gobay编写extension

1. 实现 `gobay.Extension`
2. 每一个 ext 目录都是你的例子

## 附录

## ext

### ent/orm

**关于时间**

```
// 如果需要开启 parseTime，在 dsn 中加上参数：
// 显式指定loc为UTC
parseTime=True&loc=UTC
```

> 虽然 `loc` 默认值为 `UTC`，但是这依赖于默认情况。默认情况有可能发生改变。所以我们推荐显式指定 `loc`
