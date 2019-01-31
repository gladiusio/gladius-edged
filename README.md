# Gladius Edged

The Gladius Edge Daemon serves static content to a web browser. See the main [gladius-node](https://github.com/gladiusio/gladius-node) repository to see more.

## Docker
Running the Edge Daemon in a docker container

#### From Docker Hub

```bash
$ docker run -it -v YOUR_GLADIUS_PATH:/root/.gladius \
    -p 8080:8080 gladiusio/edged:latest
```

#### Build from GitHub

```bash
$ docker build --tag=gladiusio/edged .

$ docker run -it -v $(pwd)/gladius:/root/.gladius -p 8080:8080 \
    gladiusio/edged:latest
```
* Runs the container mapping the local `./gladius` folder in this directory to the Docker container
* Exposes the content port

## Build from source

#### For your machine
You will need [Go](https://golang.org/dl/) 1.11.4 or higher (some issues with go mod checksums below that)

Run `make`. The binary will be in `./build`

#### Cross compile
Check out the [gladius-node](https://github.com/gladiusio/gladius-node) repository for Dockerized cross compilation.

## Config
Check out our [example config](./.example-config.toml) to see what values are available.