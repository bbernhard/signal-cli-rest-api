# README

This document describes how to use the Signal Messenger REST API together with Home Assistant. 

prerequisites:
* docker + docker-compose
* a phone number to send signal notifications

## Set up the docker container

* Create a `docker-compose.yml` file with the following contents: 

```
version: "3"
services:
  signal-cli-rest-api:
    image: bbernhard/signal-cli-rest-api:latest
    ports:
      - "8080:8080" # map docker port 8080 to host port 8080.
    network_mode: "host"
    volumes:
      - "./signal-cli-config:/home/.local/share/signal-cli" # map "signal-cli-config" folder on host system into docker container. the folder contains the password and cryptographic keys when a new number is registered
```

* start the docker container with `docker-compose up`


## Register phone number

In order to send signal messages to other users, you first need to register your phone number. This can be done via REST requests with: 


```curl -X POST -H "Content-Type: application/json" 'http://<ip>:<port>/v1/register/<number>'```

e.g: 

This registers the number `+431212131491291` to the Signal network.

```curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/register/+431212131491291'```

After you've sent the registration request, you will receive a token via SMS for verfication. In order to complete the registration process you need to send the verification token back via the following REST request: 

```curl -X POST -H "Content-Type: application/json" 'http://<ip>:<port>/v1/register/<number>/verify/<verification code>'```

e.g:

```curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/register/+431212131491291/verify/123-456'```


## Sending messages to Signal Messenger groups

The `signal-cli-rest-api` docker container is also capable of sending messages to a Signal Messenger group.

Requirements: 

  * Home Assistant Version >= 0.110
  * signal-cli-rest-api build-nr >= 2
    The build number can be checked with: `curl -X GET -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/about'`
  * your phone number needs to be properly registered (see the "Register phone number" section above on how to do that)

A new Signal Messenger group can be created with the following REST API request:

```curl -X POST -H "Content-Type: application/json" -d '{"name": "<name of the group>", "members": ["<member1>", "<member2>"]}' 'http://127.0.0.1:8080/v1/groups/<number>'```

e.g:

This creates a new Signal Messenger group called `my group` with the members `+4354546464654` and `+4912812812121`.

```curl -X POST -H "Content-Type: application/json" -d '{"name": "my group", "members": ["+4354546464654", "+4912812812121"]}' 'http://127.0.0.1:8080/v1/groups/+431212131491291'```

Next, use the following endpoint to obtain the group id: 

```curl -X GET -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/groups/<number>'```

The group id then needs to be added to the Signal Messenger's `recipients` list in the `configuration.yaml`. (see [here](https://www.home-assistant.io/integrations/signal_messenger/) for details)

# Troubleshooting
In case you've problems with the `signal-cli-rest-api` container, have a look [here](TROUBLESHOOTING.md)
