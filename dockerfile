# Start from the official Golang image for building
FROM golang:tip-alpine3.21 AS builder

WORKDIR /app

COPY ./src/go.mod ./src/go.sum ./
RUN go mod download

COPY ./src .

RUN CGO_ENABLED=0 GOOS=linux go build -o awx-exporter .


FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/awx-exporter .

EXPOSE 8080


ENTRYPOINT ["./awx-exporter"]