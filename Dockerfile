FROM golang:1.13-buster

ARG SIGNAL_CLI_VERSION=0.6.5

ENV GIN_MODE=release

RUN apt-get update \
	&& apt-get install -y --no-install-recommends wget default-jre software-properties-common git locales \
	&& rm -rf /var/lib/apt/lists/* 

RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8

ENV LANG en_US.UTF-8 

#RUN cd /tmp/ \
#	&& wget -P /tmp/ https://github.com/AsamK/signal-cli/archive/v${SIGNAL_CLI_VERSION}.tar.gz \
#	&& tar -xvf /tmp/v${SIGNAL_CLI_VERSION}.tar.gz \
#	&& cd signal-cli-${SIGNAL_CLI_VERSION} \
#	&& ./gradlew build \
#	&& ./gradlew installDist \
#	&& ln -s /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/install/signal-cli/bin/signal-cli /usr/bin/signal-cli \
#	&& rm -rf /tmp/v${SIGNAL_CLI_VERSION}.tar.gz

# https://github.com/AsamK/signal-cli/issues/259 is not yet in a release, so we need to check out the repository


RUN cd /tmp/ \
	&& git clone https://github.com/AsamK/signal-cli.git signal-cli-${SIGNAL_CLI_VERSION} \
	&& cd signal-cli-${SIGNAL_CLI_VERSION} \
	&& ./gradlew build \
	&& ./gradlew installDist \
	&& ln -s /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/install/signal-cli/bin/signal-cli /usr/bin/signal-cli

RUN mkdir -p /signal-cli-config/
RUN mkdir -p /home/.local/share/signal-cli
COPY src/ /tmp/signal-cli-rest-api-src
RUN cd /tmp/signal-cli-rest-api-src && go get -d ./... && go build main.go

ENV PATH /tmp/signal-cli-rest-api-src/:/usr/bin/signal-cli-${SIGNAL_CLI_VERSION}/bin/:$PATH

EXPOSE 8080

ENTRYPOINT ["main"]
