# Dockerized Signal Messenger REST API

This project creates a small dockerized REST API around [signal-cli](https://github.com/AsamK/signal-cli).

At the moment, the following functionality is exposed via REST:

- Register a number
- Verify the number using the code received via SMS
- Send message (+ attachments) to multiple recipients (or a group)
- Receive messages
- Link devices
- Create/List/Remove groups
- List/Serve/Delete attachments
- Update profile

and [many more](https://bbernhard.github.io/signal-cli-rest-api/)

## Examples

Sample `docker-compose.yml`file:

```sh
version: "3"
services:
  signal-cli-rest-api:
    image: bbernhard/signal-cli-rest-api:latest
	environment:
	  - USE_NATIVE=0
    ports:
      - "8080:8080" #map docker port 8080 to host port 8080.
    volumes:
      - "./signal-cli-config:/home/.local/share/signal-cli" #map "signal-cli-config" folder on host system into docker container. the folder contains the password and cryptographic keys when a new number is registered

```

## Native Image (EXPERIMENTAL)

On Systems like the Raspberry Pi, some operations like sending messages can take quite a while. That's because signal-cli is a Java application and a significant amount of time is spent in the JVM (Java Virtual Machine) startup. signal-cli recently added the possibility to compile the Java application to a native binary (done via GraalVM).

By adding `USE_NATIVE=1` as environmental variable to the `docker-compose.yml` file the native mode will be enabled. In case there's no native binary available (e.g on a 32 bit Raspian OS), it will fall back to the signal-cli Java application.

## API documentation

The Swagger API documentation can be found [here](https://bbernhard.github.io/signal-cli-rest-api/). If you prefer a simple text file like API documentation have a look [here](https://github.com/bbernhard/signal-cli-rest-api/blob/master/doc/EXAMPLES.md)

In case you need more functionality, please **file a ticket** or **create a PR**.
