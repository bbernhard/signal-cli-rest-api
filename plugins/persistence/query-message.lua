local http = require("http")
local json = require("json")
local sqlite = require("sqlite3").new();

function exec()
	ok, err = sqlite:open("/persistence/messages.db", { cache = "shared", mode = "rw" });
	if ok then
		res, err = sqlite:query("select data, timestamp from messages")
		if err == nil then
			for _, row in ipairs(res) do
    			row.data = json.decode(row.data)
			end
			pluginOutputData:SetPayload(json.encode(res))
			pluginOutputData:SetHttpStatusCode(200)
		else
			pluginOutputData:SetHttpStatusCode(400)
			pluginOutputData:SetPayload("Couldn't query data from sqlite db")
		end
	else
		pluginOutputData:SetHttpStatusCode(400)
		pluginOutputData:SetPayload("Couldn't query data from sqlite db")
	end
end

function init()
	ok, err = sqlite:open("/persistence/messages.db", { cache = "shared", mode = "rwc" });
	if ok then
		res, err = sqlite:exec("create table if not exists messages (id INTEGER PRIMARY KEY, data json, timestamp DATETIME DEFAULT CURRENT_TIMESTAMP)");
		if err ~= nil then
			print(err)
			return nil, err	
		end
	end	
	return nil, nil
end
