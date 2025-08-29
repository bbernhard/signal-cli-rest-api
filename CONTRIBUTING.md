# Contributing

Contributions are welcome, including bug reports, implementing features or writing documentation.

## Dev Environment Setup

The recommended way is:

1. Build the docker image, setting the version to a number of your choosing
    ```bash
    docker build --network=host --tag bbernhard/signal-cli-rest-api:<a dev version> .
    ```
2. Adjust the `docker-compose.yml`
    * Replace the `image: bbernhard/signal-cli-rest-api:latest` for `image: bbernhard/signal-cli-rest-api:<a dev version>`
    * Add the version to the environment:
        ```yml
        environment:
            - BUILD_VERSION=<a dev version>
        ```
3. Run the image
    ```bash
    docker compose up
    ```
4. Make code changes and repeat steps 1 to 3

## Building the documentation

Follow the instructions in the docs [readme page](./src/docs/README.md).