ARG SIGNAL_CLI_VERSION=0.14.3
ARG LIBSIGNAL_CLIENT_VERSION=0.92.1

ARG SWAG_VERSION=1.16.4

ARG S6_OVERLAY_VERSION=v3.2.2.0

ARG BUILD_VERSION_ARG=unset

FROM golang:1.26-trixie AS buildcontainer

ARG SIGNAL_CLI_VERSION
ARG LIBSIGNAL_CLIENT_VERSION
ARG SWAG_VERSION
ARG BUILD_VERSION_ARG

RUN dpkg-reconfigure debconf --frontend=noninteractive \
	&& apt-get update \
	&& apt-get -y install --no-install-recommends \
		wget git locales zip unzip \
		file build-essential libz-dev zlib1g-dev binutils \
	&& rm -rf /var/lib/apt/lists/*

#COPY ext/libraries/libsignal-client/v${LIBSIGNAL_CLIENT_VERSION} /tmp/libsignal-client-libraries
RUN wget https://github.com/bbernhard/libsignal-client-builds/releases/download/v${LIBSIGNAL_CLIENT_VERSION}/libsignal-client-build-v${LIBSIGNAL_CLIENT_VERSION}.tar.gz -O /tmp/libsignal-client.tar.gz
RUN cd /tmp && mkdir -p /tmp/libsignal-client-libraries && tar xf libsignal-client.tar.gz && mv x86-64 armv7 arm64 -t libsignal-client-libraries

# use architecture specific libsignal_jni.so
RUN arch="$(uname -m)"; \
        case "$arch" in \
            aarch64) cp /tmp/libsignal-client-libraries/arm64/libsignal_jni.so /tmp/libsignal_jni.so ;; \
			armv7l) cp /tmp/libsignal-client-libraries/armv7/libsignal_jni.so /tmp/libsignal_jni.so ;; \
            x86_64) cp /tmp/libsignal-client-libraries/x86-64/libsignal_jni.so /tmp/libsignal_jni.so ;; \
			*) echo "Unknown architecture" && exit 1 ;; \
        esac;

RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8

ENV JAVA_OPTS="-Djdk.lang.Process.launchMechanism=vfork"

ENV LANG=en_US.UTF-8

RUN go install github.com/swaggo/swag/cmd/swag@v${SWAG_VERSION}

RUN cd /tmp/ \
	&& wget -nv https://github.com/AsamK/signal-cli/releases/download/v${SIGNAL_CLI_VERSION}/signal-cli-${SIGNAL_CLI_VERSION}.tar.gz -O /tmp/signal-cli.tar.gz \
	&& tar xf signal-cli.tar.gz

RUN if [ "$(uname -m)" = "x86_64" ]; then \
		cd /tmp \
		&& wget https://github.com/bbernhard/signal-cli-native-builds/releases/download/v${SIGNAL_CLI_VERSION}/signal-cli-native-v${SIGNAL_CLI_VERSION}.tar.gz \
		&& tar xvf signal-cli-native-v${SIGNAL_CLI_VERSION}.tar.gz \
		&& cp signal-cli-native-v${SIGNAL_CLI_VERSION}/x86-64/signal-cli-native /tmp/signal-cli-native; \
	elif [ "$(uname -m)" = "aarch64" ] ; then \
		cd /tmp \
		&& wget https://github.com/bbernhard/signal-cli-native-builds/releases/download/v${SIGNAL_CLI_VERSION}/signal-cli-native-v${SIGNAL_CLI_VERSION}.tar.gz \
		&& tar xvf signal-cli-native-v${SIGNAL_CLI_VERSION}.tar.gz \
		&& cp signal-cli-native-v${SIGNAL_CLI_VERSION}/arm64/signal-cli-native /tmp/signal-cli-native; \
    elif [ "$(uname -m)" = "armv7l" ] ; then \
		echo "GRAALVM doesn't support 32bit" \
		&& echo "Creating temporary file, otherwise the below copy doesn't work for armv7" \
		&& mkdir -p /tmp/signal-cli-${SIGNAL_CLI_VERSION}-source/build/native/nativeCompile \
		&& touch /tmp/signal-cli-native; \
    else \
		echo "Unknown architecture"; \
    fi;

# replace libsignal-client

RUN ls /tmp/signal-cli-${SIGNAL_CLI_VERSION}/lib/libsignal-client-${LIBSIGNAL_CLIENT_VERSION}.jar || (echo "\n\nsignal-client jar file with version ${LIBSIGNAL_CLIENT_VERSION} not found. Maybe the version needs to be bumped in the signal-cli-rest-api Dockerfile?\n\n" && echo "Available version: \n" && ls /tmp/signal-cli-${SIGNAL_CLI_VERSION}/lib/libsignal-client-* && echo "\n\n" && exit 1)

# workaround until upstream is fixed
RUN cd /tmp/signal-cli-${SIGNAL_CLI_VERSION}/lib \
	&& unzip signal-cli-${SIGNAL_CLI_VERSION}.jar \
	&& sed -i 's/Signal-Android\/5.22.3/Signal-Android\/5.51.7/g' org/asamk/signal/BaseConfig.class \
	&& zip -r signal-cli-${SIGNAL_CLI_VERSION}.jar org/ META-INF/ \
	&& rm -rf META-INF \
	&& rm -rf org

RUN cd /tmp/ \
	&& zip -qu /tmp/signal-cli-${SIGNAL_CLI_VERSION}/lib/libsignal-client-${LIBSIGNAL_CLIENT_VERSION}.jar libsignal_jni.so \
	&& zip -qr signal-cli-${SIGNAL_CLI_VERSION}.zip signal-cli-${SIGNAL_CLI_VERSION}/* \
    && unzip -q /tmp/signal-cli-${SIGNAL_CLI_VERSION}.zip -d /opt \
	&& rm -f /tmp/signal-cli-${SIGNAL_CLI_VERSION}.zip

COPY src/api /tmp/signal-cli-rest-api-src/api
COPY src/client /tmp/signal-cli-rest-api-src/client
COPY src/datastructs /tmp/signal-cli-rest-api-src/datastructs
COPY src/utils /tmp/signal-cli-rest-api-src/utils
COPY src/scripts /tmp/signal-cli-rest-api-src/scripts
COPY src/main.go /tmp/signal-cli-rest-api-src/
COPY src/go.mod /tmp/signal-cli-rest-api-src/
COPY src/go.sum /tmp/signal-cli-rest-api-src/
COPY src/plugin_loader.go /tmp/signal-cli-rest-api-src/

# build signal-cli-rest-api
RUN ls -la /tmp/signal-cli-rest-api-src
RUN cd /tmp/signal-cli-rest-api-src && ${GOPATH}/bin/swag init --requiredByDefault
RUN cd /tmp/signal-cli-rest-api-src && go build -o signal-cli-rest-api main.go
RUN cd /tmp/signal-cli-rest-api-src && go test ./client -v && go test ./utils -v

# build supervisorctl_config_creator
RUN cd /tmp/signal-cli-rest-api-src/scripts && go build -o jsonrpc2-helper 

# build plugin_loader
RUN cd /tmp/signal-cli-rest-api-src && go build -buildmode=plugin -o signal-cli-rest-api_plugin_loader.so plugin_loader.go

# Start a fresh container for release container
FROM debian:trixie-slim

ARG TARGETARCH # set by buildx
ARG SIGNAL_CLI_VERSION
ARG BUILD_VERSION_ARG
ARG S6_OVERLAY_VERSION
ENV GIN_MODE=release

# Set environment variables to keep the image clean
ENV DEBIAN_FRONTEND=noninteractive

ENV PORT=8080

ENV BUILD_VERSION=$BUILD_VERSION_ARG
ENV SIGNAL_CLI_REST_API_PLUGIN_SHARED_OBJ_DIR=/usr/bin/

RUN dpkg-reconfigure debconf --frontend=noninteractive \
	&& apt-get update \
	&& apt-get install -y --no-install-recommends util-linux openjdk-25-jre curl locales xz-utils \
	&& apt-get clean \
	&& rm -rf /var/lib/apt/lists/* 

RUN if [ -z "$TARGETARCH" ]; then \
      # Fallback for older Docker versions not using BuildKit
      TARGETARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/'); \
	else \
	  echo "Building for architecture: $TARGETARCH"; \
    fi;

# install s6-overlay as service control system
RUN curl -fL -o /tmp/s6-overlay-noarch.tar.xz \
    "https://github.com/just-containers/s6-overlay/releases/download/${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz" && \
    tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz && \
    if [ "$TARGETARCH" = "amd64" ]; then S6_ARCH="x86_64"; \
    elif [ "$TARGETARCH" = "arm64" ]; then S6_ARCH="aarch64"; \
    elif [ "$TARGETARCH" = "arm" ]; then S6_ARCH="arm"; \
    else S6_ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/'); fi;\
    curl -fL -o /tmp/s6-overlay-bin.tar.xz \
    "https://github.com/just-containers/s6-overlay/releases/download/${S6_OVERLAY_VERSION}/s6-overlay-${S6_ARCH}.tar.xz" && \
    tar -C / -Jxpf /tmp/s6-overlay-bin.tar.xz && \
    rm /tmp/s6-overlay-*.tar.xz

COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/signal-cli-rest-api /usr/bin/signal-cli-rest-api
COPY --from=buildcontainer /opt/signal-cli-${SIGNAL_CLI_VERSION} /opt/signal-cli-${SIGNAL_CLI_VERSION}
COPY --from=buildcontainer /tmp/signal-cli-native /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli-native
COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/scripts/jsonrpc2-helper /usr/bin/jsonrpc2-helper
COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/signal-cli-rest-api_plugin_loader.so /usr/bin/signal-cli-rest-api_plugin_loader.so


RUN groupadd -g 1000 signal-api \
	&& useradd --no-log-init -M -d /home -s /bin/bash -u 1000 -g 1000 signal-api \
	&& ln -s /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli /usr/bin/signal-cli \
	&& ln -s /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli-native /usr/bin/signal-cli-native \
	&& mkdir -p /home/.local/share/signal-cli \
	&& chown -R signal-api:signal-api /home

COPY --chmod=755 ./s6-services/ /etc/s6-overlay/s6-rc.d/

# remove the temporary created signal-cli-native on armv7, as GRAALVM doesn't support 32bit
RUN arch="$(uname -m)"; \
        case "$arch" in \
            armv7l) echo "GRAALVM doesn't support 32bit" && rm /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli-native /usr/bin/signal-cli-native  ;; \
        esac;

RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8

ENV LANG=en_US.UTF-8

EXPOSE ${PORT}

ENV SIGNAL_CLI_CONFIG_DIR=/home/.local/share/signal-cli

RUN mkdir -p /tmp/s6-runtime && chown -R signal-api:signal-api /tmp/s6-runtime /etc/s6-overlay

USER signal-api

# Mandatory ENV for non-root s6
ENV S6_RUNTIME_PATH=/tmp/s6-runtime
ENV S6_READ_ONLY_ROOT=1
ENV S6_VERBOSITY=2

WORKDIR /home

ENTRYPOINT ["/init"]

HEALTHCHECK --interval=20s --timeout=10s --retries=3 \
    CMD curl -f http://localhost:${PORT}/v1/health || exit 1
