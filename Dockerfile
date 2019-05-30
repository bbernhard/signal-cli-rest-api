FROM ubuntu:latest

RUN apt-get update && apt-get install -y wget default-jre software-properties-common git

RUN add-apt-repository ppa:gophers/archive
RUN apt-get update
RUN apt-get install -y golang-1.11-go


# ARM
#RUN wget -P /tmp/ https://dl.google.com/go/go1.12.5.linux-armv6l.tar.gz \
#    && tar -C /usr/local -xzf /tmp/go1.12.5.linux-armv6l.tar.gz \
#    && rm -rf /tmp/go1.12.5.linux-armv6l.tar.gz 

#RUN wget -P /tmp/ https://dl.google.com/go/go1.12.5.linux-amd64.tar.gz \
#    && tar -C /user/local -xzf /tmp/go1.12.5.linux-amd64.tar.gz \
#    && rm -rf /tmp/go1.12.5.linux-amd64.tar.gz

ENV PATH /usr/lib/go-1.11/bin/:$PATH


RUN wget -P /tmp/ https://github.com/AsamK/signal-cli/releases/download/v0.6.2/signal-cli-0.6.2.tar.gz \
    && tar -C /usr/bin -xzf /tmp/signal-cli-0.6.2.tar.gz \
    && rm -rf /tmp/signal-cli-0.6.2.tar.gz


RUN mkdir -p /signal-cli-config/
RUN mkdir -p /home/.local/share/signal-cli
COPY src/ /tmp/signal-cli-rest-api-src
RUN cd /tmp/signal-cli-rest-api-src && go get -d ./... && go build main.go

ENV PATH /tmp/signal-cli-rest-api-src/:/usr/bin/signal-cli-0.6.2/bin/:$PATH

EXPOSE 8080

ENTRYPOINT ["main"]