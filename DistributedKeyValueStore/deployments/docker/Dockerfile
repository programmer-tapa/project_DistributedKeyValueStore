# Distributed Key/Value Store
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG BUILD_TARGET=kvsrvd
RUN CGO_ENABLED=0 go build -o /bin/${BUILD_TARGET} ./cmd/${BUILD_TARGET}

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
ARG BUILD_TARGET=kvsrvd
COPY --from=builder /bin/${BUILD_TARGET} /usr/local/bin/server

EXPOSE 8000 9090 9091
ENTRYPOINT ["server"]
