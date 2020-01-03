# 如何使用

```bash
# go get github.com/shanbay/gobay
gobay new
```

# 如何开发

## 更新 templates

更新完静态文件（templates目录），需要将静态文件打包成.go文件：

```bash
# go get github.com/markbates/pkger/cmd/pkger
pkger -include /cmd/gobay/templates -o cmd/gobay/
```
