# How to run a ARM64 docker on a x86-64 system

* run `docker run --rm --privileged multiarch/qemu-user-static --reset -p yes`
* pull image with `docker image pull bbernhard/signal-cli-rest-api@sha256:<SHA256 HASH>`
  (e.g: `docker image pull bbernhard/signal-cli-rest-api@sha256:e0a30fa5c2b2ff5fb21827352a0fa94c6c6fbb3a21a944cc5405c390e143e65b`; the SHA256 HASH can be found on dockerhub)
* run container with `docker run bbernhard/signal-cli-rest-api@sha256:<SHA256 HASH>`
