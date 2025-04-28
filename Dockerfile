FROM golang:1.24.2 AS builder

WORKDIR /app

COPY . .

RUN go mod tidy && go build -o main main.go

FROM alpine:latest

COPY --from=builder /app/main /app/main

CMD ["/app/main"]