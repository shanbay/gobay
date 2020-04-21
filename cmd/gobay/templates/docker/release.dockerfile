FROM alpine:3.10

COPY app /app
COPY config.yaml /config.yaml
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories
RUN apk add tzdata

ENTRYPOINT ["/app"]
