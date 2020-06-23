# Dockerized Signal Messenger REST API

This project creates a small dockerized REST API around [signal-cli](https://github.com/AsamK/signal-cli).

At the moment, the following functionality is exposed via REST:

- Register a number
- Verify the number using the code received via SMS
- Send message (+ attachment) to multiple recipients

## Examples

Sample `docker-compose.yml`file:

```
version: "3"
services:
  signal-cli-rest-api:
    image: bbernhard/signal-cli-rest-api:latest
    ports:
      - "8080:8080" #map docker port 8080 to host port 8080.
    network_mode: "host"
    volumes:
      - "./signal-cli-config:/home/.local/share/signal-cli" #map "signal-cli-config" folder on host system into docker container. the folder contains the password and cryptographic keys when a new number is registered

```

Sample REST API calls:

- Register a number (with SMS verification)

`curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/register/<number>'`

e.g:

`curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/register/+431212131491291'`

- Register a number (with voice verification)

`curl -X POST -H "Content-Type: application/json" --data '{"use_voice": true}' 'http://127.0.0.1:8080/v1/register/<number>'`

e.g:

`curl -X POST -H "Content-Type: application/json" --data '{"use_voice": true}' 'http://127.0.0.1:8080/v1/register/+431212131491291'`

- Verify the number using the code received via SMS/voice

  `curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/register/<number>/verify/<verification code>'`

  e.g:

  `curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/register/+431212131491291/verify/123-456'`

- Send a message to multiple recipients

  `curl -X POST -H "Content-Type: application/json" -d '{"message": "<message>", "number": "<number>", "recipients": ["<recipient1>", "<recipient2>"]}' 'http://127.0.0.1:8080/v2/send'`

  e.g:

  `curl -X POST -H "Content-Type: application/json" -d '{"message": "Hello World!", "number": "+431212131491291", "recipients": ["+4354546464654", "+4912812812121"]}' 'http://127.0.0.1:8080/v2/send'`

- Send a message (+ base64 encoded attachment) to multiple recipients

  `curl -X POST -H "Content-Type: application/json" -d '{"message": "<message>", "base64_attachments": ["<base64 encoded attachment>"], "number": "<number>", "recipients": ["<recipient1>", "<recipient2>"]}' 'http://127.0.0.1:8080/v2/send'`

- Send a message to a group

  The group id can be obtained via the "List groups" REST call.

  `curl -X POST -H "Content-Type: application/json" -d '{"message": "<message>", "number": "<number>", "recipients": ["<group id>"]}' 'http://127.0.0.1:8080/v2/send'`

  e.g:

  `curl -X POST -H "Content-Type: application/json" -d '{"message": "Hello World!", "number": "+431212131491291", "recipients": ["group.ckRzaEd4VmRzNnJaASAEsasa", "+4912812812121"]}' 'http://127.0.0.1:8080/v2/send'`

- Receive messages

  Fetch all new messages in the inbox of the specified number.

  `curl -X GET -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/receive/<number>'`

  e.g:

  `curl -X GET -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/receive/+431212131491291'`

- Create a new group

  Create a new group with the specified name and members.

  `curl -X POST -H "Content-Type: application/json" -d '{"name": "<group name>", "members": ["<member1>", "<member2>"]}' 'http://127.0.0.1:8080/v1/groups/<number>'`

  e.g:

  `curl -X POST -H "Content-Type: application/json" -d '{"name": "my group", "members": ["+4354546464654", "+4912812812121"]}' 'http://127.0.0.1:8080/v1/groups/+431212131491291'`

- List groups

  `curl -X GET -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/groups/<number>'`

  e.g:

  `curl -X GET -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/groups/+431212131491291'`

- Delete a group

  Delete the group with the given group id. The group id can be obtained via the "List groups" REST call.

  `curl -X DELETE -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/groups/<number>/<group id>'`

  e.g:

  `curl -X DELETE -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/groups/+431212131491291/ckRzaEd4VmRzNnJaASAEsasa'`

- Link a device

  `curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/qrcodelink?<device name>'`

  e.g:

  `curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/qrcodelink?HomeAssistant'`

  This provides a QR-Code image. In case of an error a JSON object will be returned.

  Due to security reason of Signal, the provided QR-Code will change with each request.

The following REST API endpoints are **deprecated and no longer maintained!**

`/v1/send`

In case you need more functionality, please **file a ticket** or **create a PR**
