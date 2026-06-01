ARG NODE_VERSION=20-alpine
ARG GO_VERSION=latest
ARG ALPINE_VERSION=3.20

# ---- Frontend Build ----
FROM node:${NODE_VERSION} AS frontend
RUN npm config set registry https://registry.npmmirror.com
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# ---- Backend Build ----
FROM golang:${GO_VERSION} AS backend
ENV GOPROXY=https://goproxy.cn,direct
ENV GOTOOLCHAIN=auto
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
COPY --from=frontend /app/dist embed/dist/
RUN CGO_ENABLED=0 go build -o /living-recorder .

# ---- Runtime ----
FROM alpine:${ALPINE_VERSION}
ENV TZ=Asia/Shanghai
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && \
    apk add --no-cache ffmpeg ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

EXPOSE 8080

WORKDIR /app

COPY --from=backend /living-recorder .
COPY build/config.yaml config.yaml

RUN mkdir -p /app/data /app/recordings

VOLUME ["/app/data", "/app/recordings"]

CMD ["./living-recorder"]
