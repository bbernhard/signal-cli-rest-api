local http = require("http")
local json = require("json")
local sqlite = require("sqlite3").new();

function exec()
	ok, err = sqlite:open("/persistence/messages.db", { cache = "shared", mode = "rw" });
	if ok then
		local data = json.decode(pluginInputData.payload);
		if data.params and data.params.envelope and data.params.envelope.dataMessage then
			local strippedPayload = json.encode(data.params.envelope)
			res, err = sqlite:exec("insert into messages(data) values(?)", strippedPayload)
			if err == nil then
				pluginOutputData:SetHttpStatusCode(200)
			else
				pluginOutputData:SetHttpStatusCode(400)
				pluginOutputData:SetPayload("Couldn't persist data to sqlite db")
			end
		else
			pluginOutputData:SetHttpStatusCode(200)
		end
	else
		pluginOutputData:SetHttpStatusCode(400)
		pluginOutputData:SetPayload("Couldn't persist data to sqlite db")
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
