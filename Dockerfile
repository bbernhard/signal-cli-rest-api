FROM golang:1.13-buster AS buildcontainer

ARG SIGNAL_CLI_VERSION=0.6.10
ARG SWAG_VERSION=1.6.7

ENV GIN_MODE=release

RUN apt-get update \
	&& apt-get install -y --no-install-recommends wget default-jre software-properties-common git locales \
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

RUN cd /tmp/ \
	&& git clone https://github.com/AsamK/signal-cli.git signal-cli-${SIGNAL_CLI_VERSION} \
	&& cd signal-cli-${SIGNAL_CLI_VERSION} \
	&& git checkout v${SIGNAL_CLI_VERSION} \
	&& ./gradlew build \
	&& ./gradlew installDist \
	&& ln -s /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/install/signal-cli/ /tmp/signal-cli

COPY src/api /tmp/signal-cli-rest-api-src/api
COPY src/main.go /tmp/signal-cli-rest-api-src/
COPY src/system /tmp/signal-cli-rest-api-src/system
COPY src/datastructures /tmp/signal-cli-rest-api-src/datastructures
COPY src/commands /tmp/signal-cli-rest-api-src/commands
COPY src/go.mod /tmp/signal-cli-rest-api-src/
COPY src/go.sum /tmp/signal-cli-rest-api-src/ 

RUN cd /tmp/signal-cli-rest-api-src && swag init && go build

# Start a fresh container for release container
FROM adoptopenjdk:11-jre-hotspot

RUN apt-get update\
	&& apt-get install -y --no-install-recommends dbus supervisor \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/signal-cli-rest-api /usr/bin/signal-cli-rest-api
COPY --from=buildcontainer /tmp/signal-cli /opt/signal-cli

# DBUS

# Create own signal-cli user for DBUS communication
RUN useradd -ms /bin/bash signal-cli

COPY data/org.asamk.Signal.conf /etc/dbus-1/system.d/
#COPY data/org.asamk.Signal.service /usr/share/dbus-1/system-services/
#COPY data/signal.service /etc/systemd/system/ 
COPY conf/supervisor/signal-cli.conf /etc/supervisor/conf.d/signal-cli.conf
RUN mkdir -p /var/log/signal-cli
RUN mkdir -p /var/run/dbus

RUN ln -s /opt/signal-cli/bin/signal-cli /usr/bin/signal-cli
RUN mkdir -p /signal-cli-config/
RUN mkdir -p /home/.local/share/signal-cli

EXPOSE 8080

ENTRYPOINT ["signal-cli-rest-api"]
