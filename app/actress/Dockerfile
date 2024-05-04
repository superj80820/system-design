FROM golang:1.20 as builder
WORKDIR /app
COPY ./system-design ./system-design
RUN cd system-design && go build -o ./../main app/actress/main.go
RUN rm -rf *.go

FROM debian:stable-slim
RUN apt-get update && apt-get install -y gnupg lsb-release wget ca-certificates && rm -rf /var/lib/apt/lists/*
RUN wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add - \
  && sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'
RUN apt-get update && apt-get install -y postgresql-16 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY ./system-design/app/actress/web ./web
COPY --from=builder /app/main .
CMD ["./main"]