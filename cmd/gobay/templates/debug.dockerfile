FROM alpine:3.10

COPY app /app
COPY config.yaml /config.yaml
COPY dlv /dlv

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories
RUN apk add tzdata

EXPOSE 40000
ENTRYPOINT ["/dlv", "--listen=:40000", "--headless=true", "--api-version=2", "exec", "/app"]
