FROM golang:1.26 AS builder
# base image
COPY . /src
WORKDIR /src
RUN go env -w GOPROXY=https://goproxy.cn,direct &&\
    go env -w GO111MODULE=on &&\
    make all

FROM debian:12

COPY --from=builder /src/bin /app
COPY --from=builder /src/configs/config.yaml  /app/configs/config.yaml

RUN groupadd -g 1000 ai-rag-demo && useradd -u 1000 -g 1000 -s /bin/bash ai-rag-demo && chown ai-rag-demo:ai-rag-demo /app
USER ai-rag-demo

WORKDIR /app
RUN mkdir -p /app/log
EXPOSE 8000
EXPOSE 9000

CMD ["./server"]