FROM golang:1.24-alpine AS builder

ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=off

ENV CGO_ENABLED=0 \
    GOPROXY=${GOPROXY} \
    GOSUMDB=${GOSUMDB}

WORKDIR /src
COPY native/go.mod native/go.sum /src/native/
RUN cd /src/native && go mod download
COPY native /src/native
RUN cd /src/native && \
    go build -trimpath -ldflags "-s -w -X duanjuapp/native/core.buildAllSources=true" \
    -o /out/zhenguojian-core ./server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/zhenguojian-core /usr/local/bin/zhenguojian-core

ENV PORT=8080 \
    DATA_DIR=/data \
    MAX_CONCURRENCY=8 \
    TZ=Asia/Shanghai

WORKDIR /data
VOLUME ["/data"]
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -q -O /dev/null "http://127.0.0.1:${PORT}/healthz" || exit 1

ENTRYPOINT ["/usr/local/bin/zhenguojian-core"]
