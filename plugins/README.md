Plugins allow to dynamically register custom endpoints without forking this project.

# Why?

Imagine that you want to use the Signal REST API in a software component that has some restrictions regarding the payload it supports. To give you a real world example: If you want to use the Signal REST API to send Signal notifications from your Synology NAS to your phone, the HTTP endpoint must have only "flat" parameters - i.e array parameters in the JSON payload aren't allowed. Since the `recipients` parameter in the `/v2/send` endpoint is a array parameter, the send endpoint cannot be used in the Synology NAS. In order to work around that limitation, you can write a small custom plugin in Lua, to create a custom send endpoint, which exposes a single `recipient` string instead of an array.

# How to write a custom plugin

In order to use plugins, you first need to enable that feature. This can be done by setting the environment variable `ENABLE_PLUGINS` to `true` in the `docker-compose.yml` file.

e.g:

```
services:                                                                                                                                                                                                        
  signal-cli-rest-api:
    image: bbernhard/signal-cli-rest-api:latest
    environment:
      - MODE=json-rpc #supported modes: json-rpc, native, normal
      - ENABLE_PLUGINS=true #enable plugins
``` 

A valid plugin consists of a definition file (with the file ending `.def`) and a matching lua script file (with the file ending `.lua`). Both of those files must have the same filename and are placed in a folder called `plugins` on the host filesystem. Now, bind mount the `plugins` folder from your host system into the `/plugins` folder inside the docker container. This can be done in the `docker-compose.yml` file:

```
services:
  signal-cli-rest-api:
    image: bbernhard/signal-cli-rest-api:latest
  environment:
    - MODE=json-rpc #supported modes: json-rpc, native, normal
    - ENABLE_PLUGINS=true
  volumes:
    - "./signal-cli-config:/home/.local/share/signal-cli" 
    - "./plugins:/plugins" #map "plugins" folder on host system into docker container.
```

# The definition file

The definition file (with the file suffix `.def`) contains some metadata which is necessary to properly register the new endpoint. A proper definition file looks like this:

```
endpoint: my-custom-send-endpoint/:number 
method: POST
```

The `endpoint` specifies the URI of the newly created endpoint. All custom endpoints are registered under the `/v1/plugins` endpoint. So, our `my-custom-send-endpoint` will be available at `/v1/plugins/my-custom-endpoint`. If you want to use variables inside the endpoint, prefix them with a `:`.

The `method` parameter specifies the HTTP method that is used for the endpoint registration.

# The script file

The script file (with the file suffix `.lua`) contains the implementation of the endpoint.

Example:

```
local http = require("http")
local json = require("json")

local url = "http://127.0.0.1:8080/v2/send"

local customEndpointPayload = json.decode(pluginInputData.payload)

local sendEndpointPayload = {
    recipients = {customEndpointPayload.recipient},
    message = customEndpointPayload.message,
    number = pluginInputData.Params.number
}

local encodedSendEndpointPayload = json.encode(sendEndpointPayload)

response, error_message = http.request("POST", url, {
    timeout="30s",
    headers={
        Accept="*/*",
        ["Content-Type"]="application/json"
    },
    body=encodedSendEndpointPayload
})

pluginOutputData:SetPayload(response["body"])
pluginOutputData:SetHttpStatusCode(response.status_code)
```

What the lua script does, is parse the JSON payload from the custom request, extract the `recipient` and the `message` from the payload and the `number` from the URL parameter and call the `/v2/send` endpoint with those parameters. The HTTP status code and the body that is returned by the HTTP request is then returned to the caller (this is done via the `pluginOutputData:SetPayload` and `pluginOutputData:SetHttpStatusCode` functions.

If you now invoke the following curl command, a message gets sent:

`curl -X POST -H "Content-Type: application/json" -d '{"message": "test", "recipient": "<recipient>"}' 'http://127.0.0.1:8080/v1/plugins/my-custom-send-endpoint/<registered signal number>'`

# Pass commands from/to the lua script

When a new plugin is registered, some parameters are automatically passed as global variables to the lua script:

* `pluginInputData.payload`: the (JSON) payload that is passed to the custom endpoint
* `pluginInputData.Params`: a map of all parameters that are part of the URL, which were defined in the definition file (i.e those parameters that were defined with `:` prefixed in the URL)

In order to return values from the lua script, the following functions are available:
* `pluginOutputData:SetPayload()`: Set the (JSON) payload that is returned to the caller
* `pluginOutputData:SetHttpStatusCode()`: Set the HTTP status code that is returned to the caller
