ARG SIGNAL_CLI_VERSION=0.14.1
ARG LIBSIGNAL_CLIENT_VERSION=0.87.4

ARG SWAG_VERSION=1.16.4

ARG BUILD_VERSION_ARG=unset

FROM golang:1.24-bookworm AS buildcontainer

ARG SIGNAL_CLI_VERSION
ARG LIBSIGNAL_CLIENT_VERSION
ARG SWAG_VERSION
ARG BUILD_VERSION_ARG

RUN dpkg-reconfigure debconf --frontend=noninteractive \
	&& apt-get update \
	&& apt-get -y install --no-install-recommends \
		wget software-properties-common git locales zip unzip \
	&& rm -rf /var/lib/apt/lists/*

RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8

ENV JAVA_OPTS="-Djdk.lang.Process.launchMechanism=vfork"

ENV LANG=en_US.UTF-8

RUN go install github.com/swaggo/swag/cmd/swag@v${SWAG_VERSION}

# Download pre-built signal-cli release (no source build needed for v0.14.x)
RUN cd /tmp/ \
	&& wget -nv https://github.com/AsamK/signal-cli/releases/download/v${SIGNAL_CLI_VERSION}/signal-cli-${SIGNAL_CLI_VERSION}.tar.gz -O /tmp/signal-cli.tar.gz \
	&& tar xf signal-cli.tar.gz

# Extract the platform-specific native libsignal_jni.so from the release jar
# and re-inject it at the root of the jar (where signal-cli expects it at runtime).
# The release jar bundles per-platform .so files under resource paths; we extract
# the correct one for the build architecture and place it at the jar root.
RUN cd /tmp/ \
	&& arch="$(uname -m)"; \
	   case "$arch" in \
	       aarch64) so_name=libsignal_jni_aarch64.so ;; \
	       x86_64)  so_name=libsignal_jni_amd64.so ;; \
	       *)       echo "Unsupported architecture: $arch" && exit 1 ;; \
	   esac; \
	   unzip -jo /tmp/signal-cli-${SIGNAL_CLI_VERSION}/lib/libsignal-client-${LIBSIGNAL_CLIENT_VERSION}.jar "$so_name" -d /tmp/ \
	&& mv /tmp/$so_name /tmp/libsignal_jni.so \
	&& zip -qu /tmp/signal-cli-${SIGNAL_CLI_VERSION}/lib/libsignal-client-${LIBSIGNAL_CLIENT_VERSION}.jar libsignal_jni.so

RUN cp -r /tmp/signal-cli-${SIGNAL_CLI_VERSION} /opt/signal-cli-${SIGNAL_CLI_VERSION}

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
RUN cd /tmp/signal-cli-rest-api-src && ${GOPATH}/bin/swag init
RUN cd /tmp/signal-cli-rest-api-src && go build -o signal-cli-rest-api main.go
RUN cd /tmp/signal-cli-rest-api-src && go test ./client -v && go test ./utils -v

# build supervisorctl_config_creator
RUN cd /tmp/signal-cli-rest-api-src/scripts && go build -o jsonrpc2-helper

# build plugin_loader
RUN cd /tmp/signal-cli-rest-api-src && go build -buildmode=plugin -o signal-cli-rest-api_plugin_loader.so plugin_loader.go

# Start a fresh container for release container
FROM eclipse-temurin:25-jre-noble

ENV GIN_MODE=release

ENV PORT=8080

ARG SIGNAL_CLI_VERSION
ARG BUILD_VERSION_ARG

ENV BUILD_VERSION=$BUILD_VERSION_ARG
ENV SIGNAL_CLI_REST_API_PLUGIN_SHARED_OBJ_DIR=/usr/bin/

RUN dpkg-reconfigure debconf --frontend=noninteractive \
	&& apt-get update \
	&& apt-get install -y --no-install-recommends util-linux supervisor netcat-openbsd curl locales \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/signal-cli-rest-api /usr/bin/signal-cli-rest-api
COPY --from=buildcontainer /opt/signal-cli-${SIGNAL_CLI_VERSION} /opt/signal-cli-${SIGNAL_CLI_VERSION}
COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/scripts/jsonrpc2-helper /usr/bin/jsonrpc2-helper
COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/signal-cli-rest-api_plugin_loader.so /usr/bin/signal-cli-rest-api_plugin_loader.so
COPY entrypoint.sh /entrypoint.sh


RUN (userdel ubuntu 2>/dev/null; groupdel ubuntu 2>/dev/null; true) \
	&& groupadd -g 1000 signal-api \
	&& useradd --no-log-init -M -d /home -s /bin/bash -u 1000 -g 1000 signal-api \
	&& ln -sf /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli /usr/bin/signal-cli \
	&& mkdir -p /signal-cli-config/ \
	&& mkdir -p /home/.local/share/signal-cli

RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8

ENV LANG=en_US.UTF-8

EXPOSE ${PORT}

ENV SIGNAL_CLI_CONFIG_DIR=/home/.local/share/signal-cli
ENV SIGNAL_CLI_UID=1000
ENV SIGNAL_CLI_GID=1000
ENV SIGNAL_CLI_CHOWN_ON_STARTUP=true

ENTRYPOINT ["/entrypoint.sh"]

HEALTHCHECK --interval=20s --timeout=10s --retries=3 \
    CMD curl -f http://localhost:${PORT}/v1/health || exit 1
