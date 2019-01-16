FROM golang:1.11.4

WORKDIR /app

COPY . .

VOLUME /root/.gladius

RUN go build -o app -i cmd/gladius-edged/main.go

CMD ["./app"]