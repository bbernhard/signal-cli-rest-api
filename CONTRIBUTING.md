# Contributing

Contributions are welcome, including bug reports, implementing features or writing documentation.

## Dev Environment Setup

The recommended way is to use docker compose:

1. In the `docker-compose.yml` replace the `image: bbernhard/signal-cli-rest-api:latest` for `build: "."`

The development workflow is:

1. Make changes in the source code.
2. Build the image and run a container.
    ```bash
    docker compose build
    ```
    ```bash
    docker compose up
    ```
3. Test the changes.
4. Bring the container down.
    ```bash
    docker compose down
    ```
4. Repeat as many times as needed.

## Building the documentation

Follow the instructions in the docs [readme page](./src/docs/README.md).