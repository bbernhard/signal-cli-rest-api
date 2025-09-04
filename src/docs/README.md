These files are generated from the [swaggo/swag](https://github.com/swaggo/swag) tool.

To regenerate them, run in /src:

```bash
docker run --rm -v $(pwd):/code ghcr.io/swaggo/swag:latest init
```

Or, if you have `swag` installed:

```bash
swag init
```

Then run the app in `/src`

```bash
go run main.go
```

Or with docker compose in the root of the repository
```bash
docker compose up
```

Then navigate to the following address to view the docs

http://127.0.0.1:8080/swagger/index.html

On docker you'll get a Network error, replace the IP for the docker internal IP in the error, e.g:

http://172.18.0.2:8080/swagger/index.html