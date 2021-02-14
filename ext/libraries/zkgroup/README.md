# HOWTO BUILD

[cross](https://github.com/rust-embedded/cross) is used for cross compiling zkgroup.

* download new release from `https://github.com/signalapp/zkgroup/releases`
* unzip + change into directory
* run `cross build --target x86_64-unknown-linux-gnu --release`
  
  run `cross build --target armv7-unknown-linux-gnueabihf --release`
  
  run `cross build --target aarch64-unknown-linux-gnu --release`
to build the library for `x86-64`, `armv7` and `arm64`
* the built library will be in the `target/<architecture>/release` folder 

## Why?

Building zkgroup every time a new docker image gets released takes really long (especially for cross platform builds with docker/buildx and QEMU). Furthermore, due to this bug here (https://github.com/docker/buildx/issues/395) we would need to use an ugly workaround for that right now. As zkgroup isn't released very often I guess it's okay to manually build a new version once in a while.  
