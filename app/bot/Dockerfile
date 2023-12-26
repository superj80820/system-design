FROM golang:1.20 as builder
WORKDIR /app
COPY . .
RUN go build -ldflags '-extldflags "-static"' -o main app/bot/main.go
RUN rm -rf *.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/main .
CMD ["./main"]