FROM golang:1.13-buster as base-image
# Global ENV & ARG (for multistaging)
ENV CONTAINER_ENCODING="UTF-8"
ENV CONTAINER_LANGUAGE="en_US"
ENV SIGNAL_CLI_DIR=/app/signal-cli
ENV API_DIR=/app/api
ENV SIGNAL_CLI_BIN="/tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/install/signal-cli/bin/signal-cli"
ENV API_BIN="/tmp/signal-cli-rest-api-src/main"
ENV LANG ${CONTAINER_LANGUAGE}.${CONTAINER_ENCODING}
ARG SIGNAL_CLI_VERSION=0.6.5
# end of base-image section

FROM base-image as build
ENV GIN_MODE=release

RUN apt-get update \
	&& apt-get install -y --no-install-recommends wget default-jre software-properties-common git \
	&& rm -rf /var/lib/apt/lists/* 

RUN cd /tmp/ \
	&& git clone https://github.com/AsamK/signal-cli.git signal-cli-${SIGNAL_CLI_VERSION} \
	&& cd signal-cli-${SIGNAL_CLI_VERSION} \
	&& ./gradlew build \
	&& ./gradlew installDist \
	&& ln -s /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/install/signal-cli/bin/signal-cli /usr/bin/signal-cli

#RUN cd /tmp/ \
#	&& wget -P /tmp/ https://github.com/AsamK/signal-cli/archive/v${SIGNAL_CLI_VERSION}.tar.gz \
#	&& tar -xvf /tmp/v${SIGNAL_CLI_VERSION}.tar.gz \
#	&& cd signal-cli-${SIGNAL_CLI_VERSION} \
#	&& ./gradlew build \
#	&& ./gradlew installDist \
#	&& ln -s /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/install/signal-cli/bin/signal-cli /usr/bin/signal-cli \
#	&& rm -rf /tmp/v${SIGNAL_CLI_VERSION}.tar.gz

# https://github.com/AsamK/signal-cli/issues/259 is not yet in a release, so we need to check out the repository

###
# RUN mkdir -p /signal-cli-config/
# COPY src/ /tmp/signal-cli-rest-api-src
# RUN cd /tmp/signal-cli-rest-api-src && go get -d ./... && go build main.go
###

FROM base-image as prod

COPY --from=build ${SIGNAL_CLI_BIN} /usr/bin/signal-cli
COPY --from=build ${API_BIN} /usr/bin/main
RUN apt-get update \
	&& apt-get install -y --no-install-recommends locales default-jre \
	&& rm -rf /var/lib/apt/lists/* 


COPY entrypoint.sh /tmp/entrypoint.sh
RUN chmod 755 /tmp/entrypoint.sh
RUN echo ${SIGNAL_CLI_BIN}
EXPOSE 8080

ENTRYPOINT ["/tmp/entrypoint.sh"]

CMD ["/usr/bin/main"]
