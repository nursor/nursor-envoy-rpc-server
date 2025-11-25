FROM golang:1.24.2 AS builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main main.go

FROM alpine:latest

COPY --from=builder /app/main /app/main

CMD ["/app/main"]