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


## Getting started

1. Create a directory for the configuration
This allows you to update `signal-cli-rest-api` by just deleting and recreating the container without the need to re-register your signal number

```bash
$ mkdir $HOME/.local/share/signal-cli
```


2. Start a container

```bash
$ sudo docker run -d --name signal-api --restart=always -p 8080:8080 \
      -v $HOME/.local/share/signal-cli:/home/.local/share/signal-cli \
      -e 'MODE=native' bbernhard/signal-cli-rest-api
```

3. Register or Link your Signal Number

In this case we'll register our container as secondary device, assuming that you already have your primary number running / assigned to your mobile. 

Therefore open http://localhost:8080/v1/qrcodelink?device_name=signal-api in your browser, open Signal on your mobile phone, go to _Settings > Linked devices_ and scan the QR code using the _+_ button.

4. Test your new REST API

Call the REST API endpoint and send a test message: Replace `+4412345` with your signal number in international number format, and `+44987654` with the recipients number.

```bash
$ curl -X POST -H "Content-Type: application/json" 'http://localhost:8080/v2/send' \
     -d '{"message": "Test via Signal API!", "number": "+4412345", "recipients": [ "+44987654" ]}' 
```

You should now have send a message to `+44987654`.

## Execution Modes 

The `signal-cli-rest-api` supports three different modes of execution, which can be controlled by setting the `MODE` environment variable.

* **`normal` Mode: (Default)** The `signal-cli` executable is invoked for every REST API request. Being a Java application, each REST call requires a new startup of the JVM (Java Virtual Machine), increasing the latency and hence leading to the slowest mode of operation. 
* **`native` Mode:** A precompiled binary `signal-cli-native` (using GraalVM) is used for every REST API request. This results in a much lower latency & memory usage on each call. On the `armv7` platform this mode is not available and falls back to `normal`. The native mode may also be less stable, due to the experimental state of GraalVM compiler. 
* `json-rpc` Mode: A single, JVM-based `signal-cli` instance is spawned as daemon process. This mode is usually the fastest, but requires more memory as the JVM keeps running. 


|     mode     |    speed    |    resident memory usage |
|-------------:|:------------|:------------|
|   `normal`    |    :heavy_check_mark:       | normal
|   `native`    |    :heavy_check_mark: :heavy_check_mark:    | normal
|   `json-rpc`  |    :heavy_check_mark: :heavy_check_mark: :heavy_check_mark: | increased


**Example of running `signal-cli-rest` in `native` mode**

```bash
$ sudo docker run -d --name signal-api --restart=always -p 9922:8080 \
              -v /home/user/signal-api:/home/.local/share/signal-cli \
              -e 'MODE=native' bbernhard/signal-cli-rest-api
```

This launches a instance of the REST service accessible under http://localhost:9922/v2/send. To preserve the Signal number registration, i.e. for updates, the storage location for the `signal-cli` configuration is mapped as Docker Volume into a local `/home/user/signal-api` directory.


## Auto Receive Schedule

> :warning: This setting is only needed in normal/native mode!

[signal-cli](https://github.com/AsamK/signal-cli), which this REST API wrapper is based on, recommends to call `receive` on a regular basis. So, if you are not already calling the `receive` endpoint regularily, it is recommended to set the `AUTO_RECEIVE_SCHEDULE` parameter in the docker-compose.yml file. The `AUTO_RECEIVE_SCHEDULE` accepts cron schedule expressions and automatically calls the `receive` endpoint at the given time. e.g: `0 22 * * *` calls `receive` daily at 10pm. If you are not familiar with cron schedule expressions, you can use this [website](https://crontab.guru).

**WARNING** Calling `receive` will fetch all the messages for the registered Signal number from the Signal Server! So, if you are using the REST API for receiving messages, it's _not_ a good idea to use the `AUTO_RECEIVE_SCHEDULE` parameter, as you might lose some messages that way.

## Example

Sample `docker-compose.yml`file:

```yaml
version: "3"
services:
  signal-cli-rest-api:
    image: bbernhard/signal-cli-rest-api:latest
    environment:
      - MODE=normal #supported modes: json-rpc, native, normal
      #- AUTO_RECEIVE_SCHEDULE=0 22 * * * #enable this parameter on demand (see description below)
    ports:
      - "8080:8080" #map docker port 8080 to host port 8080.
    volumes:
      - "./signal-cli-config:/home/.local/share/signal-cli" #map "signal-cli-config" folder on host system into docker container. the folder contains the password and cryptographic keys when a new number is registered
```

## Documentation

### API Reference

The Swagger API documentation can be found [here](https://bbernhard.github.io/signal-cli-rest-api/). If you prefer a simple text file based API documentation have a look [here](https://github.com/bbernhard/signal-cli-rest-api/blob/master/doc/EXAMPLES.md).

### Blog Posts

[Running Signal Messenger REST API in Azure Web App for Containers](https://stefanstranger.github.io/2021/06/01/RunningSignalRESTAPIinAppService/) by [@stefanstranger](https://github.com/stefanstranger)

## Advanced Settings
There are a bunch of environmental variables that can be set inside the docker container in order to change some technical details. This settings are meant for developers and advanced users. Usually you do *not* need to change anything here - the default values are perfectly fine!

* `SIGNAL_CLI_CONFIG_DIR`: Specifies the path to the `signal-cli` config directory inside the docker container. Defaults to `/home/.local/share/signal-cli/`

* `SIGNAL_CLI_UID`: Specifies the uid of the `signal-api` user inside the docker container. Defaults to `1000`

* `SIGNAL_CLI_GID`: Specifies the gid of the `signal-api` group inside the docker container. Defaults to `1000`

* `SWAGGER_IP`: The IP that's used in the Swagger UI for the interactive examples. Defaults to the container ip.

## Clients & Libraries

|     Name    | Client           | Library  | Language | Maintainer |
| ------------- |:-------------:| :-----:|:-----:|:-----:|
| [pysignalclirestapi](https://pypi.org/project/pysignalclirestapi/)      | | X | Python | [@bbernhard](https://github.com/bbernhard)
| [signalbot](https://pypi.org/project/signalbot/) | | X | Python | [@filipre](https://github.com/filipre)

In case you need more functionality, please **file a ticket** or **create a PR**.
