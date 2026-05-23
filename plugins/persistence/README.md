# Persistence Plugin

Plugin which writes every received message to a sqlite3 database.

## Howto enable this plugin

* Download the `persist-message.def`, `persist-message.lua`, `query-message.def` and `query-message.lua` files and put them in a `plugins` folder on your filesystem
* Create a `persistence` folder on your host system. In this folder the docker container then creates the sqlite3 database.
* Adapt your `docker-compose.yml` to enable the plugin and map the required resources into the docker container

```
services:                                                                                                                                                                                                        
  signal-cli-rest-api:
    image: bbernhard/signal-cli-rest-api:latest
    environment:
      - MODE=json-rpc #supported modes: json-rpc, native, normal (choose the mode you want; the plugin works with all modes)
      - ENABLE_PLUGINS=true # enable plugins
      - "./plugins:/plugins" #map "plugins" folder from the host system into the docker container
      - "./persistence;/persistence" #map "persistence" folder from the host system into the docker container
      - RECEIVE_WEBHOOK_URL=http://127.0.0.1:8080/v1/plugins/persistence/persist-message #register an internal webhook endpoint
```
* Restart your docker container

Every message that is received is then written to the `messages.db` inside the `persistence` folder.

The stored messages can then be received via the REST API with:

`curl -X GET 'http://127.0.0.1:8080/v1/plugins/persistence/query-message'`

## Debugging and Troubleshooting

* Make sure that the docker container has write permissions to the `persistence` folder
* On the host system, check if the `messages.db` gets created in the `persistence` folder
* Check the logs. Do you see any error?

