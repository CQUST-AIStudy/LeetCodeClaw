ARG GO_IMAGE=docker.m.daocloud.io/library/golang:1.25-alpine
ARG ALPINE_IMAGE=docker.m.daocloud.io/library/alpine:3.22
ARG ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
ARG GOPROXY=https://goproxy.cn,direct

FROM ${GO_IMAGE} AS builder

ARG ALPINE_MIRROR
ARG GOPROXY

WORKDIR /src

RUN sed -i "s|https://dl-cdn.alpinelinux.org/alpine|${ALPINE_MIRROR}|g" /etc/apk/repositories \
	&& apk add --no-cache ca-certificates tzdata

ENV GOPROXY=${GOPROXY}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/leetcode-api ./cmd/leetcode-api

FROM ${ALPINE_IMAGE}

ARG ALPINE_MIRROR

RUN sed -i "s|https://dl-cdn.alpinelinux.org/alpine|${ALPINE_MIRROR}|g" /etc/apk/repositories \
	&& apk add --no-cache ca-certificates tzdata wget \
	&& addgroup -S app \
	&& adduser -S app -G app

WORKDIR /app

COPY --from=builder /out/leetcode-api /app/leetcode-api

ENV LEETCODE_CLAW_ADDR=:10170
ENV TZ=Asia/Shanghai

EXPOSE 10170

USER app

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
	CMD wget -q -O /dev/null http://127.0.0.1:10170/ready || exit 1

ENTRYPOINT ["/app/leetcode-api"]
