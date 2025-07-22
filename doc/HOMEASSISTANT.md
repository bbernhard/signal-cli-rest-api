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

### Trouble shooting: if you get the response `Binary output can mess up your terminal`
If you get the following response back from from linking a new number to the API

```
Warning: Binary output can mess up your terminal. Use "--output -" to tell 
Warning: curl to output it to your terminal anyway, or consider "--output 
Warning: <FILE>" to save to a file.
```
Execute the same API call again and add `--output img.jpg` to save the QR code response back as a image file. Then scan the QR code from the signal mobile app signed in using the required mobile number. 

```sh
curl -X GET -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/qrcodelink?device_name=HomeAssistant' --output img.jpg
```

### Trouble shooting: Number is locked with a PIN
If you are trying to verify a number that has a PIN assigned to it, you will get an error message saying: "Verification failed! This number is locked with a pin". You can provide the PIN using "--data '{"pin": "your registration lock pin"}'" to the `curl` verification call:

```sh
curl -X POST -H "Content-Type: application/json" --data '{"pin": "your registration lock pin"}' 'http://127.0.0.1:8080/v1/register/+431212131491291/verify/123-456'
```

### Trouble shooting: A captcha is required
If, in step 1 above, you receive a response like `{"error":"Captcha required for verification (null)\n"}` then Signal is requiring a captcha. To register the number you must do the following (in Mozilla Firefox):
1. Go to  [https://signalcaptchas.org/registration/generate.html](https://signalcaptchas.org/registration/generate.html)
2. Solve the captcha
3. Once successful, a "Open Signal" button will appear below the solved captcha, right click it and select "Copy Link" or "Copy link address". 
4. Paste what you just copied as `<captcha value>` in the example below. (The content will be a very long string starting with something like `signalcaptcha://signal-hcaptcha...`)

```sh
curl -X POST -H "Content-Type: application/json" -d '{"captcha":"<captcha value>", "use_voice": false}' 'http://127.0.0.1:8080/v1/register/<number>'
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

You can use Signal Messenger REST API as a Home Assistant trigger. In this example, we will make a simple chatbot. If you write "time" to your Signal account linked to Signal Messenger REST API, the automation gets triggered, with the condition that the number (attribute source) is correct, to take action by sending a Signal notification back with the current time: now().

To accomplish this, edit the configuration of Home Assistant, adding a [RESTful resource](https://www.home-assistant.io/integrations/rest/) as follows:

```yaml
- resource: "http://127.0.0.1:8080/v1/receive/<number>"
  headers:
    Content-Type: application/json
  sensor:
    - name: "Signal message received"
      value_template: "{{ value_json[0].envelope.dataMessage.message }}" #this will fetch the message
      json_attributes_path: $[0].envelope
      json_attributes:
       - source #using attributes you can get additional information, in this case the phone number.
  ```
You can create an automation as follows:

```yaml
...
trigger:
  - platform: state
    entity_id:
      - sensor.signal_message_received
    to: time
condition:
  - condition: state
    entity_id: sensor.signal_message_received
    attribute: source
    state: "<yournumber>"
action:
  - service: notify.signal
    data:
      message: "{{ now() }}"
```

## API details

Details regarding API (in example for receiving messages through REST) can be found [here](https://bbernhard.github.io/signal-cli-rest-api/)

## Troubleshooting

In case you've problems with the `signal-cli-rest-api` container, have a look [here](TROUBLESHOOTING.md)
