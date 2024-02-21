FROM golang:1.20 as builder
WORKDIR /app
COPY ./system-design ./system-design
RUN cd system-design && go build -o ./../main app/exchange-gitbitex/main.go
RUN rm -rf *.go

FROM debian:stable-slim
WORKDIR /app
COPY --from=builder /app/main .
COPY ./system-design/app/exchange-gitbitex .
CMD ["./main"]