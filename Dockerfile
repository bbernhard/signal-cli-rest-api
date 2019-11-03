FROM golang:1.13-buster

RUN apt-get update \
	&& apt-get install -y --no-install-recommends wget default-jre software-properties-common git \
	&& rm -rf /var/lib/apt/lists/* 

RUN wget -P /tmp/ https://github.com/AsamK/signal-cli/releases/download/v0.6.4/signal-cli-0.6.4.tar.gz \
    && tar -C /usr/bin -xzf /tmp/signal-cli-0.6.4.tar.gz \
    && rm -rf /tmp/signal-cli-0.6.4.tar.gz


RUN mkdir -p /signal-cli-config/
RUN mkdir -p /home/.local/share/signal-cli
COPY src/ /tmp/signal-cli-rest-api-src
RUN cd /tmp/signal-cli-rest-api-src && go get -d ./... && go build main.go

ENV PATH /tmp/signal-cli-rest-api-src/:/usr/bin/signal-cli-0.6.2/bin/:$PATH

EXPOSE 8080

ENTRYPOINT ["main"]
