ARG SIGNAL_CLI_VERSION=0.11.4
ARG LIBSIGNAL_CLIENT_VERSION=0.20.0
ARG SIGNAL_CLI_NATIVE_PACKAGE_VERSION=0.11.4-1

ARG SWAG_VERSION=1.6.7
ARG GRAALVM_JAVA_VERSION=17
ARG GRAALVM_VERSION=22.1.0

ARG BUILD_VERSION_ARG=unset

FROM golang:1.17-bullseye AS buildcontainer

ARG SIGNAL_CLI_VERSION
ARG LIBSIGNAL_CLIENT_VERSION
ARG SWAG_VERSION
ARG GRAALVM_JAVA_VERSION
ARG GRAALVM_VERSION
ARG BUILD_VERSION_ARG
ARG SIGNAL_CLI_NATIVE_PACKAGE_VERSION

COPY ext/libraries/libsignal-client/v${LIBSIGNAL_CLIENT_VERSION} /tmp/libsignal-client-libraries

# use architecture specific libsignal_jni.so
RUN arch="$(uname -m)"; \
        case "$arch" in \
            aarch64) cp /tmp/libsignal-client-libraries/arm64/libsignal_jni.so /tmp/libsignal_jni.so ;; \
			armv7l) cp /tmp/libsignal-client-libraries/armv7/libsignal_jni.so /tmp/libsignal_jni.so ;; \
            x86_64) cp /tmp/libsignal-client-libraries/x86-64/libsignal_jni.so /tmp/libsignal_jni.so ;; \
			*) echo "Unknown architecture" && exit 1 ;; \
        esac;

RUN dpkg-reconfigure debconf --frontend=noninteractive \
	&& apt-get -qq update \
	&& apt-get -qqy install --no-install-recommends \
		wget openjdk-17-jre software-properties-common git locales zip unzip \
		file build-essential libz-dev zlib1g-dev < /dev/null > /dev/null \
	&& rm -rf /var/lib/apt/lists/* 

RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8

ENV JAVA_OPTS="-Djdk.lang.Process.launchMechanism=vfork"

ENV LANG en_US.UTF-8

RUN cd /tmp/ \
	&& git clone https://github.com/swaggo/swag.git swag-${SWAG_VERSION} \	
	&& cd swag-${SWAG_VERSION} \
	&& git checkout -q v${SWAG_VERSION} \
	&& make -s < /dev/null > /dev/null \
	&& cp /tmp/swag-${SWAG_VERSION}/swag /usr/bin/swag \
	&& rm -r /tmp/swag-${SWAG_VERSION}

RUN cd /tmp/ \
	&& wget -nv https://github.com/AsamK/signal-cli/releases/download/v${SIGNAL_CLI_VERSION}/signal-cli-${SIGNAL_CLI_VERSION}-Linux.tar.gz -O /tmp/signal-cli.tar.gz \
	&& tar xf signal-cli.tar.gz

# build native image with graalvm

RUN arch="$(uname -m)"; \
        case "$arch" in \
            aarch64) wget -nv https://github.com/graalvm/graalvm-ce-builds/releases/download/vm-${GRAALVM_VERSION}/graalvm-ce-java${GRAALVM_JAVA_VERSION}-linux-aarch64-${GRAALVM_VERSION}.tar.gz -O /tmp/gvm.tar.gz ;; \
            armv7l) echo "GRAALVM doesn't support 32bit" ;; \
            x86_64) wget -nv https://github.com/graalvm/graalvm-ce-builds/releases/download/vm-${GRAALVM_VERSION}/graalvm-ce-java${GRAALVM_JAVA_VERSION}-linux-amd64-${GRAALVM_VERSION}.tar.gz -O /tmp/gvm.tar.gz ;; \ 
			*) echo "Invalid architecture" ;; \
        esac;

RUN if [ "$(uname -m)" = "x86_64" ]; then \
		cd /tmp \
		&& git clone https://github.com/AsamK/signal-cli.git signal-cli-${SIGNAL_CLI_VERSION}-source \
		&& cd signal-cli-${SIGNAL_CLI_VERSION}-source \
		&& git checkout -q v${SIGNAL_CLI_VERSION} \
		&& cd /tmp && tar xf gvm.tar.gz \
		&& export GRAALVM_HOME=/tmp/graalvm-ce-java${GRAALVM_JAVA_VERSION}-${GRAALVM_VERSION} \
		&& export PATH=/tmp/graalvm-ce-java${GRAALVM_JAVA_VERSION}-${GRAALVM_VERSION}/bin:$PATH \
		&& cd /tmp/signal-cli-${SIGNAL_CLI_VERSION}-source \
		&& sed -i 's/Signal-Android\/5.22.3/Signal-Android\/5.51.7/g' src/main/java/org/asamk/signal/BaseConfig.java \
		&& chmod +x /tmp/graalvm-ce-java${GRAALVM_JAVA_VERSION}-${GRAALVM_VERSION}/bin/gu \ 
		&& /tmp/graalvm-ce-java${GRAALVM_JAVA_VERSION}-${GRAALVM_VERSION}/bin/gu install native-image \
		&& ./gradlew -q nativeCompile; \
	elif [ "$(uname -m)" = "aarch64" ] ; then \
		echo "Use native image from @morph027 (https://packaging.gitlab.io/signal-cli/) for arm64 - many thanks to @morph027" \
		&& curl -fsSL https://packaging.gitlab.io/signal-cli/gpg.key | apt-key add - \
		&& echo "deb https://packaging.gitlab.io/signal-cli focal main" > /etc/apt/sources.list.d/morph027-signal-cli.list \
		&& mkdir -p /tmp/signal-cli-native \
		&& cd /tmp/signal-cli-native \
		&& wget https://gitlab.com/packaging/signal-cli/-/jobs/2730692987/artifacts/download?file_type=archive -O /tmp/signal-cli-native/archive.zip \
		&& unzip archive.zip \
		#&& apt-get -qq update \
		#&& apt-get -qq download signal-cli-native=${SIGNAL_CLI_NATIVE_PACKAGE_VERSION} < /dev/null > /dev/null \
		&& ar x *.deb \
		&& tar xf data.tar.gz \
		&& mkdir -p /tmp/signal-cli-${SIGNAL_CLI_VERSION}-source/build/native/nativeCompile \
		&& cp /tmp/signal-cli-native/usr/bin/signal-cli-native  /tmp/signal-cli-${SIGNAL_CLI_VERSION}-source/build/native/nativeCompile/signal-cli; \
    elif [ "$(uname -m)" = "armv7l" ] ; then \
		echo "GRAALVM doesn't support 32bit" \
		&& echo "Creating temporary file, otherwise the below copy doesn't work for armv7" \
		&& mkdir -p /tmp/signal-cli-${SIGNAL_CLI_VERSION}-source/build/native/nativeCompile \
		&& touch /tmp/signal-cli-${SIGNAL_CLI_VERSION}-source/build/native/nativeCompile/signal-cli; \
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
COPY src/utils /tmp/signal-cli-rest-api-src/utils
COPY src/scripts /tmp/signal-cli-rest-api-src/scripts
COPY src/main.go /tmp/signal-cli-rest-api-src/
COPY src/go.mod /tmp/signal-cli-rest-api-src/
COPY src/go.sum /tmp/signal-cli-rest-api-src/

# build signal-cli-rest-api
RUN cd /tmp/signal-cli-rest-api-src && swag init && go test ./client -v && go build

# build supervisorctl_config_creator
RUN cd /tmp/signal-cli-rest-api-src/scripts && go build -o jsonrpc2-helper 

# Start a fresh container for release container
FROM eclipse-temurin:17-focal

ENV GIN_MODE=release

ENV PORT=8080

ARG SIGNAL_CLI_VERSION
ARG BUILD_VERSION_ARG

ENV BUILD_VERSION=$BUILD_VERSION_ARG

RUN dpkg-reconfigure debconf --frontend=noninteractive \
	&& apt-get -qq update \
	&& apt-get -qq install -y --no-install-recommends util-linux supervisor netcat < /dev/null > /dev/null \
	&& rm -rf /var/lib/apt/lists/* 

COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/signal-cli-rest-api /usr/bin/signal-cli-rest-api
COPY --from=buildcontainer /opt/signal-cli-${SIGNAL_CLI_VERSION} /opt/signal-cli-${SIGNAL_CLI_VERSION}
COPY --from=buildcontainer /tmp/signal-cli-${SIGNAL_CLI_VERSION}-source/build/native/nativeCompile/signal-cli /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli-native
COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/scripts/jsonrpc2-helper /usr/bin/jsonrpc2-helper
COPY entrypoint.sh /entrypoint.sh


RUN groupadd -g 1000 signal-api \
	&& useradd --no-log-init -M -d /home -s /bin/bash -u 1000 -g 1000 signal-api \
	&& ln -s /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli /usr/bin/signal-cli \
	&& ln -s /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli-native /usr/bin/signal-cli-native \
	&& mkdir -p /signal-cli-config/ \
	&& mkdir -p /home/.local/share/signal-cli

# remove the temporary created signal-cli-native on armv7, as GRAALVM doesn't support 32bit
RUN arch="$(uname -m)"; \
        case "$arch" in \
            armv7l) echo "GRAALVM doesn't support 32bit" && rm /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli-native /usr/bin/signal-cli-native  ;; \
			aarch64) echo "GRAALVM temporarily disabled for aarch64" && rm /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli-native /usr/bin/signal-cli-native  ;; \
        esac;

EXPOSE ${PORT}

ENV SIGNAL_CLI_CONFIG_DIR=/home/.local/share/signal-cli
ENV SIGNAL_CLI_UID=1000
ENV SIGNAL_CLI_GID=1000

ENTRYPOINT ["/entrypoint.sh"]

HEALTHCHECK --interval=20s --timeout=10s --retries=3 \
    CMD curl -f http://localhost:${PORT}/v1/health || exit 1
