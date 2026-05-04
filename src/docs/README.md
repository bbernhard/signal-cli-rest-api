# Documentation

These files are generated using the [swaggo/swag](https://github.com/swaggo/swag) tool.

There are two steps, first generating the docs and then running the web server.

## With docker compose (recommended)

1. Build the docs 
    ```bash
    docker compose build
    ```
2. Serve the docs
    ```bash
    docker compose up
    ```
3. Go to http://127.0.0.1:8080/swagger/index.html to view the docs
    * If you get a Network error, replace the IP for the docker internal IP in the error, e.g: http://172.18.0.2:8080/swagger/index.html

## Locally

Install [go](https://go.dev/).

### Generating the docs

1. Set the current working dir to `src`
    ```bash
    cd src
    ```
1. Run swag to generate the docs
    * Option 1, via go
        ```bash
        go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --requiredByDefault --outputTypes "go,json"
        ```
    * Option 2, directly with swag
        ```bash
        swag init --requiredByDefault --outputTypes "go,json"
        ```
    * Option 3, swag via docker 
        ```bash
        docker run --rm -v $(pwd):/code ghcr.io/swaggo/swag:latest init --requiredByDefault --outputTypes "go,json"
        ```
1. Set the current working dir to `src/docs`
    ```bash
    cd docs
    ```
1. Run the script to add the receive V1 schemas from https://github.com/AsamK/signal-cli#json-schemas-for-the-json-rpc-mode
    ```bash
    go run add_v1_receive_schemas.go ./path-to-signal-cli-json-schema-folder
    ```

### Run the web server

Run the web server to visualize the generated docs.

1. Navigate to the `src` folder
    ```bash
    cd src
    ```
1. Run the main script
    ```bash
    go run main.go
    ```
1. Go to http://127.0.0.1:8080/swagger/index.html
