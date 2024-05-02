FROM golang:1.20 as builder
WORKDIR /app
COPY ./system-design ./system-design
RUN cd system-design && go build -o ./../main app/actress/main.go
RUN rm -rf *.go

FROM debian:stable-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY ./system-design/app/actress/web ./web
COPY --from=builder /app/main .
CMD ["./main"]