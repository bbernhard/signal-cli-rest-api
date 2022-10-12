# HOWTO BUILD

[cross](https://github.com/rust-embedded/cross) is used for cross compiling [libsignal-client](https://github.com/signalapp/libsignal-client).

* download new release from `https://github.com/signalapp/libsignal-client/releases`
* unzip + change into directory
* cd into `java` directory
* run `cross build --target x86_64-unknown-linux-gnu --release -p libsignal-jni`
  
  run `cross build --target armv7-unknown-linux-gnueabihf --release -p libsignal-jni`
  
  run `cross build --target aarch64-unknown-linux-gnu --release -p libsignal-jni`
to build the library for `x86-64`, `armv7` and `arm64`
* the built library will be in the `target/<architecture>/release` folder 

## Why?

Building libsignal-client every time a new docker image gets released takes really long (especially for cross platform builds with docker/buildx and QEMU). Furthermore, due to this bug here (https://github.com/docker/buildx/issues/395) we would need to use an ugly workaround for that right now. As libsignal-client isn't released very often I guess it's okay to manually build a new version once in a while.  

## Pitfalls

1. `cross` requires a Rust installation via [`rustup`](https://www.rust-lang.org/tools/install), otherwise you might receive a _toolchain is not fully qualified_-Error.
2. Your user must have permission to directly talk to the Docker daemon using the `docker` command. To avoid running as root, [you can add your user to the docker group and reboot](https://docs.docker.com/engine/install/linux-postinstall/). 
