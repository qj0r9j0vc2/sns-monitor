# Stage 1 - Build
FROM golang:1.23.4 AS builder

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV GOAMD64=v1

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o sns-monitor ./cmd

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/sns-monitor .

EXPOSE 8080

ENTRYPOINT ["./sns-monitor"]
