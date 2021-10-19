# README

This document describes how to use the Signal Messenger REST API together with Octoprint.

prerequisites:

* docker + docker-compose
* a phone number to send signal notifications

## Set up the docker container

* Create a `docker-compose.yml` file with the following contents:

```sh
version: "3"
services:
  signal-cli-rest-api:
    image: bbernhard/signal-cli-rest-api:latest
    environment:
      - MODE=json-rpc #supported modes: json-rpc, native, normal
    ports:
      - "8080:8080" # map docker port 8080 to host port 8080.
    volumes:
      - "./signal-cli-config:/home/.local/share/signal-cli" # map "signal-cli-config" folder on host system into docker container. the folder contains the password and cryptographic keys when a new number is registered
```

* start the docker container with `docker-compose up`

## Register phone number

In order to send signal messages to other users, you first need to register your phone number. This can be done via REST requests with:

`curl -X POST -H "Content-Type: application/json" 'http://<ip>:<port>/v1/register/<number>'`

e.g:

This registers the number `+431212131491291` to the Signal network.

`curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/register/+431212131491291'`

After you've sent the registration request, you will receive a token via SMS for verfication. In order to complete the registration process you need to send the verification token back via the following REST request:

```curl -X POST -H "Content-Type: application/json" 'http://<ip>:<port>/v1/register/<number>/verify/<verification code>'```

e.g:

```curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/register/+431212131491291/verify/123-456'```

## Troubleshooting

In case you've problems with the `signal-cli-rest-api` container, have a look [here](TROUBLESHOOTING.md)
