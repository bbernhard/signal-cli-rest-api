This project is just a small REST API Wrapper around [signal-cli](https://github.com/AsamK/signal-cli) - i.e all the heavy lifting is done by `signal-cli`. In case you are experiencing some problems, it's important to determine whether the problems are caused by the REST API wrapper or by `signal-cli`.

This can be done by putting the docker container into debug mode with the following REST API command:

```curl -X POST -H "Content-Type: application/json" -d '{"logging": {"level": "debug"}}' 'http://127.0.0.1:8080/v1/configuration'```

Once the docker container is in debug mode, execute the REST API command you want to debug. 

e.g Let's assume we are experiencing some problems with sending messages. So, let's send a Signal message with  

```curl -X POST -H "Content-Type: application/json" -d '{"message": "Hello World!", "number": "+431212131491291", "recipients": ["+4354546464654", "+4912812812121"]}' 'http://127.0.0.1:8080/v2/send'```

and see what the docker container is doing internally with the request.

In the docker-compose log file are now all the steps listed you need to perform in oder to use `signal-cli` directly without the REST API wrapper involved.

e.g for the above request we would see the following lines in the docker-compose log file:

```
signal-cli-rest-api_1  | time="2021-06-17T07:41:33Z" level=debug msg="If you want to run this command manually, run the following steps on your host system:"
signal-cli-rest-api_1  | time="2021-06-17T07:41:33Z" level=debug msg="*) docker exec -it 2cb5036847fd07c47100c34c5b7b3b2f38c78a449e6dd20833d1662b32a6713a /bin/bash"
signal-cli-rest-api_1  | time="2021-06-17T07:41:33Z" level=debug msg="*) su signal-api"
signal-cli-rest-api_1  | time="2021-06-17T07:41:33Z" level=debug msg="*) echo 'Hello World!' | signal-cli --config /home/.local/share/signal-cli -u +431212131491291 send +4354546464654 +4912812812121"
```

By removing the REST API wrapper from the equation and using `signal-cli` directly it's possible to determine whether the issue is caused by the REST API wrapper or `signal-cli`.
In case it doesn't work when using `signal-cli` directly, it's most probably an issue with `signal-cli` and you need to file a ticket in the [signal-cli repository](https://github.com/AsamK/signal-cli).


Before you create a ticket in the `signal-cli` repository, please make sure to collect as much information as possible to make it easier for the maintainer to reproduce your bug:

* Post the obfuscated(!) `signal-cli` command you tried and the obfuscated(!) output you got (please also add the `--verbose` flag to the `signal-cli` command in order to get more debugging output. see: https://github.com/AsamK/signal-cli/blob/master/man/signal-cli.1.adoc)
* Do you see any patterns? Does the issue only occur with specific phone numbers?
* Have you tried to do a `receive` first? `signal-cli` recommends to run this command periodically. 
* ...
