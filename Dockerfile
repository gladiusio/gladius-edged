# build stage
FROM golang:1.11.4 AS build-env
ADD . /src
RUN cd /src && go build -tags netgo -a -v -o gladius-edged -i cmd/gladius-edged/main.go

# final stage
FROM alpine
RUN apk --no-cache --update upgrade && apk --no-cache add ca-certificates
WORKDIR /app
VOLUME /root/.gladius
COPY --from=build-env /src/gladius-edged /app/
ENTRYPOINT ./gladius-edged