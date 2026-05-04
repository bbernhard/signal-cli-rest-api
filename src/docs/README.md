# Documentation

These files are generated using the [swaggo/swag](https://github.com/swaggo/swag) tool.

There are two steps, first generating the docs and then running the web server.

## Generating the docs

Regenerate the files with your local source code changes.

1. Set the current working dir to `src`
    ```bash
    cd src
    ```
1. Run swag to generate the docs
    * Option 1, via docker 
        ```bash
        docker run --rm -v $(pwd):/code ghcr.io/swaggo/swag:latest init --requiredByDefault --outputTypes go
        ```
    * Option 2, install swag and run the command line tool
        ```bash
        swag init --requiredByDefault --outputTypes go
        ```
1. Set the current working dir to `src/docs`
    ```bash
    cd docs
    ```
1. Run the script to add the receive V1 schemas
    ```bash
    go run add_v1_receive_schemas.go
    ```

## Run the web server

Run the web server to visualize the generated docs.

1. Run the main script
    * Option 1, via docker, run the command at the root of the repository
        ```bash
        docker compose up
        ```
    * Option 2, install go and run the command line tool
        ```bash
        cd src
        ```
        ```bash
        go run main.go
        ```


## Navigate to the docs

The docs are served at: http://127.0.0.1:8080/swagger/index.html

When serving with docker, if you get a Network error, replace the IP for the docker internal IP in the error, e.g: http://172.18.0.2:8080/swagger/index.html