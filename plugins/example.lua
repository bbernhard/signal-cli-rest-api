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
print(encodedSendEndpointPayload)

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
