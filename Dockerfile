ARG SWAG_VERSION=1.6.7

ARG BUILD_VERSION_ARG=unset

FROM golang:1.17-bullseye AS buildcontainer

ARG SWAG_VERSION
ARG BUILD_VERSION_ARG

RUN apt-get update \
	&& apt-get install -y --no-install-recommends wget software-properties-common git locales zip file build-essential libz-dev zlib1g-dev \
	&& rm -rf /var/lib/apt/lists/* 

RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8

ENV LANG en_US.UTF-8

RUN cd /tmp/ \
	&& git clone https://github.com/swaggo/swag.git swag-${SWAG_VERSION} \	
	&& cd swag-${SWAG_VERSION} \
	&& git checkout v${SWAG_VERSION} \
	&& make \
	&& cp /tmp/swag-${SWAG_VERSION}/swag /usr/bin/swag \
	&& rm -r /tmp/swag-${SWAG_VERSION}

COPY src/api /tmp/signal-cli-rest-api-src/api
COPY src/client /tmp/signal-cli-rest-api-src/client
COPY src/utils /tmp/signal-cli-rest-api-src/utils
COPY src/scripts /tmp/signal-cli-rest-api-src/scripts
COPY src/main.go /tmp/signal-cli-rest-api-src/
COPY src/go.mod /tmp/signal-cli-rest-api-src/
COPY src/go.sum /tmp/signal-cli-rest-api-src/

# build signal-cli-rest-api
RUN cd /tmp/signal-cli-rest-api-src && swag init && go build

# build supervisorctl_config_creator
RUN cd /tmp/signal-cli-rest-api-src/scripts && go build -o jsonrpc2-helper 

# Start a fresh container for release container
FROM ubuntu:focal

ENV GIN_MODE=release

ENV PORT=8080

ARG SIGNAL_CLI_VERSION
ARG BUILD_VERSION_ARG

ENV BUILD_VERSION=$BUILD_VERSION_ARG

RUN apt-get update \
	&& apt-get install -y --no-install-recommends \
		util-linux \
		supervisor \
		netcat \
		unzip \
		apt-transport-https \
		ca-certificates \
	&& rm -rf /var/lib/apt/lists/* 

ADD	--chown=_apt:root https://packaging.gitlab.io/signal-cli/gpg.key /etc/apt/trusted.gpg.d/morph027-signal-cli.asc

RUN	echo "deb https://packaging.gitlab.io/signal-cli focal main" > /etc/apt/sources.list.d/morph027-signal-cli.list

RUN	apt-get update \
	&& apt-get install --no-install-recommends -y \
		signal-cli-jre=${SIGNAL_CLI_VERSION}* \
	&& apt-get autoremove -y \
	&& apt-get clean \
	&& rm -rf /var/lib/apt/lists/*

# do not install signal-cli-native on armv7, as GRAALVM doesn't support 32bit
RUN arch="$(uname -m)"; \
        case "$arch" in \
            armv7l) echo "GRAALVM doesn't support 32bit" ;; \
			*) apt-get install -y --no-install-recommends signal-cli-native=${SIGNAL_CLI_VERSION}*; apt-get autoremove -y; apt-get clean; rm -rf /var/lib/apt/lists/* ;; \
        esac;

COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/signal-cli-rest-api /usr/bin/signal-cli-rest-api
COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/scripts/jsonrpc2-helper /usr/bin/jsonrpc2-helper
COPY entrypoint.sh /entrypoint.sh

RUN groupadd -g 1000 signal-api \
	&& useradd --no-log-init -M -d /home -s /bin/bash -u 1000 -g 1000 signal-api \
	&& mkdir -p /signal-cli-config/ \
	&& mkdir -p /home/.local/share/signal-cli

EXPOSE ${PORT}

ENV SIGNAL_CLI_CONFIG_DIR=/home/.local/share/signal-cli

ENTRYPOINT ["/entrypoint.sh"]

HEALTHCHECK --interval=20s --timeout=10s --retries=3 \
    CMD curl -f http://localhost:${PORT}/v1/health || exit 1
