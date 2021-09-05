ARG SIGNAL_CLI_VERSION=0.8.5
ARG ZKGROUP_VERSION=0.7.0
ARG LIBSIGNAL_CLIENT_VERSION=0.8.1

ARG SWAG_VERSION=1.6.7
ARG GRAALVM_JAVA_VERSION=11
ARG GRAALVM_VERSION=21.0.0

FROM golang:1.14-buster AS buildcontainer

ARG SIGNAL_CLI_VERSION
ARG ZKGROUP_VERSION
ARG LIBSIGNAL_CLIENT_VERSION
ARG SWAG_VERSION
ARG GRAALVM_JAVA_VERSION
ARG GRAALVM_VERSION

COPY ext/libraries/zkgroup/v${ZKGROUP_VERSION} /tmp/zkgroup-libraries
COPY ext/libraries/libsignal-client/v${LIBSIGNAL_CLIENT_VERSION} /tmp/libsignal-client-libraries

# use architecture specific libzkgroup.so
RUN arch="$(uname -m)"; \
        case "$arch" in \
            aarch64) echo "[DEBUG] Using arm64 zkgroup" && cp /tmp/zkgroup-libraries/arm64/libzkgroup.so /tmp/libzkgroup.so ;; \
			armv7l) echo "[DEBUG] Using armv7 zkgroup" && cp /tmp/zkgroup-libraries/armv7/libzkgroup.so /tmp/libzkgroup.so ;; \
            x86_64) echo "[DEBUG] Using x86-64 zkgroup" && cp /tmp/zkgroup-libraries/x86-64/libzkgroup.so /tmp/libzkgroup.so ;; \ 
			*) echo "Unknown architecture" && exit 1 ;; \
        esac;

# use architecture specific libsignal_jni.so
RUN arch="$(uname -m)"; \
        case "$arch" in \
            aarch64) cp /tmp/libsignal-client-libraries/arm64/libsignal_jni.so /tmp/libsignal_jni.so ;; \
			armv7l) cp /tmp/libsignal-client-libraries/armv7/libsignal_jni.so /tmp/libsignal_jni.so ;; \
            x86_64) cp /tmp/libsignal-client-libraries/x86-64/libsignal_jni.so /tmp/libsignal_jni.so ;; \ 
			*) echo "Unknown architecture" && exit 1 ;; \
        esac;

RUN apt-get update \
	&& apt-get install -y --no-install-recommends wget default-jre software-properties-common git locales zip file build-essential libz-dev zlib1g-dev \
	&& rm -rf /var/lib/apt/lists/* 

RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8

ENV PATH="/root/.cargo/bin:${PATH}"

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
	&& cp /tmp/libzkgroup.so ./lib/src/main/resources/libzkgroup.so \
	&& cp /tmp/libsignal_jni.so ./lib/src/main/resources/libsignal_jni.so \
	&& ./gradlew build \
	&& ./gradlew installDist \
	&& ./gradlew distTar

# build native image with graalvm

RUN arch="$(uname -m)"; \
        case "$arch" in \
            aarch64) wget https://github.com/graalvm/graalvm-ce-builds/releases/download/vm-${GRAALVM_VERSION}/graalvm-ce-java${GRAALVM_JAVA_VERSION}-linux-aarch64-${GRAALVM_VERSION}.tar.gz -O /tmp/gvm.tar.gz ;; \
            armv7l) echo "GRAALVM doesn't support 32bit" ;; \
            x86_64) wget https://github.com/graalvm/graalvm-ce-builds/releases/download/vm-${GRAALVM_VERSION}/graalvm-ce-java${GRAALVM_JAVA_VERSION}-linux-amd64-${GRAALVM_VERSION}.tar.gz -O /tmp/gvm.tar.gz ;; \ 
        esac;

RUN if [ "$(uname -m)" = "aarch64" ] || [ "$(uname -m)" = "x86_64" ]; then \
		cd /tmp && tar xvf gvm.tar.gz \
		&& export GRAALVM_HOME=/tmp/graalvm-ce-java${GRAALVM_JAVA_VERSION}-${GRAALVM_VERSION} \
		&& cd /tmp/signal-cli-${SIGNAL_CLI_VERSION} \
		&& chmod +x /tmp/graalvm-ce-java${GRAALVM_JAVA_VERSION}-${GRAALVM_VERSION}/bin/gu \ 
		&& /tmp/graalvm-ce-java${GRAALVM_JAVA_VERSION}-${GRAALVM_VERSION}/bin/gu install native-image \
		&& ./gradlew assembleNativeImage; \
    elif [ "$(uname -m)" = "armv7l" ]; then \
		echo "GRAALVM doesn't support 32bit" \
		&& echo "Creating temporary file, otherwise the below copy doesn't work for armv7" \
		&& mkdir -p /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/native-image \
		&& touch /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/native-image/signal-cli; \ 
    else \
		echo "Unknown architecture"; \
    fi;

# replace zkgroup

RUN ls /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/install/signal-cli/lib/zkgroup-java-${ZKGROUP_VERSION}.jar || (echo "\n\nzkgroup jar file with version ${ZKGROUP_VERSION} not found. Maybe the version needs to be bumped in the signal-cli-rest-api Dockerfile?\n\n" && echo "Available version: \n" && ls /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/install/signal-cli/lib/zkgroup-java-* && echo "\n\n" && exit 1)

RUN cd /tmp/ \
	&& zip -u /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/install/signal-cli/lib/zkgroup-java-${ZKGROUP_VERSION}.jar libzkgroup.so 

RUN cd /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/distributions/ \
	&& mkdir -p signal-cli-${SIGNAL_CLI_VERSION}/lib/ \
	&& cp /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/install/signal-cli/lib/zkgroup-java-${ZKGROUP_VERSION}.jar signal-cli-${SIGNAL_CLI_VERSION}/lib/ \
	# update zip
	&& zip -u /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/distributions/signal-cli-${SIGNAL_CLI_VERSION}.zip signal-cli-${SIGNAL_CLI_VERSION}/lib/zkgroup-java-${ZKGROUP_VERSION}.jar \	
	# update tar
	&& tar --delete -vPf /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/distributions/signal-cli-${SIGNAL_CLI_VERSION}.tar signal-cli-${SIGNAL_CLI_VERSION}/lib/zkgroup-java-${ZKGROUP_VERSION}.jar \
	&& tar --owner='' --group='' -rvPf /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/distributions/signal-cli-${SIGNAL_CLI_VERSION}.tar signal-cli-${SIGNAL_CLI_VERSION}/lib/zkgroup-java-${ZKGROUP_VERSION}.jar

# replace libsignal-client

RUN ls /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/install/signal-cli/lib/signal-client-java-${LIBSIGNAL_CLIENT_VERSION}.jar || (echo "\n\nsignal-client jar file with version ${LIBSIGNAL_CLIENT_VERSION} not found. Maybe the version needs to be bumped in the signal-cli-rest-api Dockerfile?\n\n" && echo "Available version: \n" && ls /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/install/signal-cli/lib/signal-client-java-* && echo "\n\n" && exit 1)

RUN cd /tmp/ \
	&& zip -u /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/install/signal-cli/lib/signal-client-java-${LIBSIGNAL_CLIENT_VERSION}.jar libsignal_jni.so

RUN cd /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/distributions/ \
	&& mkdir -p signal-cli-${SIGNAL_CLI_VERSION}/lib/ \
	&& cp /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/install/signal-cli/lib/signal-client-java-${LIBSIGNAL_CLIENT_VERSION}.jar signal-cli-${SIGNAL_CLI_VERSION}/lib/ \
	# update zip
	&& zip -u /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/distributions/signal-cli-${SIGNAL_CLI_VERSION}.zip signal-cli-${SIGNAL_CLI_VERSION}/lib/signal-client-java-${LIBSIGNAL_CLIENT_VERSION}.jar \	
	# update tar
	&& tar --delete -vPf /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/distributions/signal-cli-${SIGNAL_CLI_VERSION}.tar signal-cli-${SIGNAL_CLI_VERSION}/lib/signal-client-java-${LIBSIGNAL_CLIENT_VERSION}.jar \
	&& tar --owner='' --group='' -rvPf /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/distributions/signal-cli-${SIGNAL_CLI_VERSION}.tar signal-cli-${SIGNAL_CLI_VERSION}/lib/signal-client-java-${LIBSIGNAL_CLIENT_VERSION}.jar


COPY src/api /tmp/signal-cli-rest-api-src/api
COPY src/client /tmp/signal-cli-rest-api-src/client
COPY src/utils /tmp/signal-cli-rest-api-src/utils
COPY src/main.go /tmp/signal-cli-rest-api-src/
COPY src/go.mod /tmp/signal-cli-rest-api-src/
COPY src/go.sum /tmp/signal-cli-rest-api-src/

RUN cd /tmp/signal-cli-rest-api-src && swag init && go build


# Start a fresh container for release container
FROM adoptopenjdk:11-jre-hotspot-bionic

ENV GIN_MODE=release

ENV PORT=8080

ARG SIGNAL_CLI_VERSION

RUN apt-get update \
	&& apt-get install -y --no-install-recommends setpriv \
	&& rm -rf /var/lib/apt/lists/* 

COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/signal-cli-rest-api /usr/bin/signal-cli-rest-api
COPY --from=buildcontainer /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/distributions/signal-cli-${SIGNAL_CLI_VERSION}.tar /tmp/signal-cli-${SIGNAL_CLI_VERSION}.tar
COPY --from=buildcontainer /tmp/signal-cli-${SIGNAL_CLI_VERSION}/build/native-image/signal-cli /tmp/signal-cli-native
COPY entrypoint.sh /entrypoint.sh

RUN tar xf /tmp/signal-cli-${SIGNAL_CLI_VERSION}.tar -C /opt
RUN rm -rf /tmp/signal-cli-${SIGNAL_CLI_VERSION}.tar

RUN groupadd -g 1000 signal-api \
	&& useradd --no-log-init -M -d /home -s /bin/bash -u 1000 -g 1000 signal-api \
	&& ln -s /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli /usr/bin/signal-cli \
	&& cp /tmp/signal-cli-native /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli-native \
	&& ln -s /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli-native /usr/bin/signal-cli-native \
	&& rm /tmp/signal-cli-native \
	&& mkdir -p /signal-cli-config/ \
	&& mkdir -p /home/.local/share/signal-cli

# remove the temporary created signal-cli-native on armv7, as GRAALVM doesn't support 32bit
RUN arch="$(uname -m)"; \
        case "$arch" in \
            armv7l) echo "GRAALVM doesn't support 32bit" && rm /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli-native /usr/bin/signal-cli-native  ;; \
        esac;

EXPOSE ${PORT}

ENV SIGNAL_CLI_CONFIG_DIR=/home/.local/share/signal-cli

ENTRYPOINT ["/entrypoint.sh"]

HEALTHCHECK --interval=20s --timeout=10s --retries=3 \
    CMD curl -f http://localhost:${PORT}/v1/health || exit 1
