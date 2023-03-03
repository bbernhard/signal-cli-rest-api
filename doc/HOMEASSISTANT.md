# README

This document describes how to use the Signal Messenger REST API together with Home Assistant.

This document covers the following topics:
* [Installation](#installation)
* [Set up a phone number](#set-up-a-phone-number)
* [Sending messages to Signal Messenger groups](#sending-messages-to-signal-messenger-groups)
* [Sending messages to Signal to trigger events](#sending-messages-to-signal-to-trigger-events)

See also the [documentation of the Home Assistant integration](https://www.home-assistant.io/integrations/signal_messenger/).

## Installation

### Prerequisites:

* infrastructure to run this container on:
  * alternative 1: Docker (+ Docker Compose for ease of configuration)
  * alternative 2: ability to install Home Assistant add-ons
* a phone number to send signal notifications from

### Alternative 1: Set up the Docker container

* Create a `docker-compose.yml` file with the following contents:

```yaml
version: "3"
services:
  signal-cli-rest-api:
    image: bbernhard/signal-cli-rest-api:latest
    environment:
      - MODE=json-rpc #supported modes: json-rpc, native, normal. json-prc is recommended for speed
    ports:
      - "8080:8080" # map docker port 8080 to host port 8080.
    volumes:
      - "./signal-cli-config:/home/.local/share/signal-cli" # map "signal-cli-config" folder on host system into docker container. the folder contains the password and cryptographic keys when a new number is registered
```

* start the docker container with `docker-compose up`

### Alternative 2: Install Home Assistant Add-on

Add this repository to your Home Assistant Add-on Store repository list:

[https://github.com/haberda/hassio_addons](https://github.com/haberda/hassio_addons)

Then install and start the add-on.

## Set up a phone number
You will need to set up the phone number(s) to send notifications from using `REST` request. (You can set up multiple numbers.) In order to do so, make sure, you have `curl` installed on your system. You can then issue the commands shown below from the command line. We use the example server `127.0.0.1` on port `8080`. If you have set up a different server, you will have to change this in the commands.

For trouble shooting of common problems during setup, see below.

### Alternatives

* You can use a new phone number. This is recommended, as it enables full feature support. Land-line numbers are supported.
* You can link `signal-cli-rest-api` as a secondary device to an existing account on your mobile phone.

### Alternative 1: Register a new phone number
1. In order to send signal messages to other users, you first need to register your phone number. This can be done via REST requests with:

   **Note**: If you want to register a land-line number, set the `use_voice` parameter to `true`. Signal will then call you on your number and speak the token to you instead of sending an SMS.

   ```sh
   curl -X POST -H "Content-Type: application/json" --data '{"use_voice": false}' 'http://<ip>:<port>/v1/register/<number>'
   ```

   **Example**: The following command registers the number `+431212131491291` to the Signal network.

   ```sh
   curl -X POST -H "Content-Type: application/json" --data '{"use_voice": false}' 'http://127.0.0.1:8080/v1/register/+431212131491291'
   ```

2. After you've sent the registration request, you will receive a token via SMS (or it will be spoken to you) for verfication.

3. In order to complete the registration process, you need to send the verification token back via the following REST request:

   ```sh
   curl -X POST -H "Content-Type: application/json" 'http://<ip>:<port>/v1/register/<number>/verify/<verification code>'
   ```

   **Example**: The following will send a verification code for the previous example number.

   ```sh
   curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/register/+431212131491291/verify/123-456'
   ```

### Alternative 2: Use your existing mobile phone number
It is recommended to use a fresh number. Some things might not work as expected, if you only link the REST API to an existing number. For example, if you send a notification to a group including yourself, you will not be notified. This is, because the notification is sent by yourself. Therefore, consider registering your land-line number for connecting your home to Signal.

1. To link the REST API as a new device to an existing account on your mobile phone, send the following command:

   ```sh
   curl -X GET -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/qrcodelink?device_name=<device name>'
   ```

   **Example**:

   ```sh
   curl -X GET -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/qrcodelink?device_name=HomeAssistant'
   ```

2. This provides a QR-Code image. In case of an error a JSON object will be returned.

   Due to security reason of Signal, the provided QR-Code will change with each request.
   
3. Scan the QR-Code with your main Signal app.

4. The REST API will be linked to your main account. You can use it then to send message on your own personal behalf.

### Trouble shooting: Number is locked with a PIN
If you are trying to verify a number that has a PIN assigned to it, you will get an error message saying: "Verification failed! This number is locked with a pin". You can provide the PIN using "--data '{"pin": "your registration lock pin"}'" to the `curl` verification call:

```sh
curl -X POST -H "Content-Type: application/json" --data '{"pin": "your registration lock pin"}' 'http://127.0.0.1:8080/v1/register/+431212131491291/verify/123-456'
```

### Trouble shooting: A captcha is required
If, in step 1 above, you receive a response like `{"error":"Captcha required for verification (null)\n"}` then Signal is requiring a captcha. To register the number you must do the following (in Mozilla Firefox):
1. Go to  [https://signalcaptchas.org/registration/generate.html](https://signalcaptchas.org/registration/generate.html)
2. Open the developer console
3. Answer the captcha
3. On the developer console, find the line that looks like this: `Prevented navigation to “signalcaptcha://{captcha value}” due to an unknown protocol.` Copy the captcha value
4. Use it to make the registration call like this:

```sh
curl -X POST -H "Content-Type: application/json" -d '{"captcha":"captcha value", "use_voice": false}' 'http://127.0.0.1:8080/v1/register/<number>'
```

## Sending messages to Signal Messenger groups

The `signal-cli-rest-api` docker container is also capable of sending messages to a Signal Messenger group.

For trouble shooting of common problems during setup, see below.

### Requirements

* Home Assistant Version >= 0.110
* `signal-cli-rest-api` build-nr >= 2
  The build number can be checked with: `curl -X GET -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/about'`
* your phone number needs to be properly registered (see the "Register phone number" section above on how to do that)

### Create a new group

1. A new Signal Messenger group can be created with the following REST API request:

   ```sh
   curl -X POST -H "Content-Type: application/json" -d '{"name": "<name of the group>", "members": ["<member1>", "<member2>"]}' 'http://127.0.0.1:8080/v1/groups/<number>'
   ```

   **Example**: The following creates a new Signal Messenger group called `my group` with the members `+4354546464654` and `+4912812812121`.

   ```sh
   curl -X POST -H "Content-Type: application/json" -d '{"name": "my group", "members": ["+4354546464654", "+4912812812121"]}' 'http://127.0.0.1:8080/v1/groups/+431212131491291'
   ```

2. Next, use the following endpoint to obtain the `group_id`:

   ```sh
   curl -X GET -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/groups/<number>'
   ```

3. The `group_id` then needs to be added to the Signal Messenger's `recipients` list in the `configuration.yaml`. (see [here](https://www.home-assistant.io/integrations/signal_messenger/) for details)

### Trouble shooting: If you receive an empty group list

In order for groups to show up in the `groups` list, try the following steps:

* Use the JSON-RPC mode. This might already solve your issue.
* You might need to first `receive` data from the Signal servers. (See https://github.com/AsamK/signal-cli/issues/82.)

  To do so, use the following REST request:
  
  ```sh
  curl -X GET -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/receive/<number>'
  ```

## Sending messages to Signal to trigger events
In this example, we will be using signal-cli-rest-api to as a Home Assistant trigger. For example, you would be able to write 'light' to your Signal account linked to signal-cli-rest-api and have Home Assistant adjust the lights for you. To accomplish this, you will need to edit the `rest.yaml` configuration of Home Assistant, add the following resource:
```- resource: "http://127.0.0.1:8080/v1/receive/<number>"
  headers:
    Content-Type: application/json
  sensor:
    - name: "Signal message received"
      value_template: '{{ value_json[0].envelope.dataMessage.message }}'
  ```
And then you can create an automation as follows:
```alias: "[signal] message received"
trigger:
  - platform: state
    entity_id:
      - sensor.signal_message_received
condition: []
action:
  - if:
      - condition: state
        entity_id: sensor.signal_message_received
        state: <word to use for trigger>
    then:
      - service: <service to run>
        data: {}
mode: single
```


## API details

Details regarding API (in example for receiving messages through REST) can be found [here](https://bbernhard.github.io/signal-cli-rest-api/)

## Troubleshooting

In case you've problems with the `signal-cli-rest-api` container, have a look [here](TROUBLESHOOTING.md)
