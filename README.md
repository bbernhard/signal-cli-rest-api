# Dockerized signal-cli REST API

This project creates a small dockerized REST API around [signal-cli](https://github.com/AsamK/signal-cli).


At the moment, the following functionality is exposed via REST: 

* Register a number
* Verify the number using the code received via SMS
* Send message (+ attachment) to multiple recipients


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

* Register a number (with SMS verification)

```curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/register/<number>'```

   e.g:
   
   ```curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/register/+431212131491291'```

* Verify the number using the code received via SMS

   ```curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/register/<number>/verify/<verification code>'```

   e.g:
   
   ```curl -X POST -H "Content-Type: application/json" 'http://127.0.0.1:8080/v1/register/+431212131491291/verify/123-456'```

* Send message to multiple recipients

   ```curl -X POST -H "Content-Type: application/json" -d '{"message": "<message>", "number": "<number>", "recipients": ["<recipient1>", "<recipient2>"]}' 'http://127.0.0.1:8080/v1/send'```

   e.g:

   ```curl -X POST -H "Content-Type: application/json" -d '{"message": "Hello World!", "number": "+431212131491291", "recipients": ["+4354546464654", "+4912812812121"]}' 'http://127.0.0.1:8080/v1/send'```

* Send a message (+ base64 encoded attachment) to multiple recipients 

  ```curl -X POST -H "Content-Type: application/json" -d '{"message": "<message>", "base64_attachment": "<base64 encoded attachment>", "number": "<number>", "recipients": ["<recipient1>", "<recipient2>"]}' 'http://127.0.0.1:8080/v1/send'```

In case you need more functionality, please **create a pull request**
