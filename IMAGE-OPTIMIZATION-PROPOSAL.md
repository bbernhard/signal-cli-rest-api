# Image Size Optimization Proposal

## Problem

The current Docker image is **1.33 GB** and bundles two complete signal-cli runtimes that no single `MODE` ever uses simultaneously:

| Layer | Size | Used by |
|---|---|---|
| ubuntu:noble base | 78 MB | all |
| openjdk-25-jre + deps | **655 MB** | `normal`, `json-rpc` |
| signal-cli Java dist (jars) | 121 MB | `normal`, `json-rpc` |
| signal-cli-native binary | **389 MB** | `native`, `json-rpc-native` |
| Go binary (signal-cli-rest-api) | 40 MB | all |
| plugin_loader.so | 34 MB | all |
| jsonrpc2-helper | 9 MB | json-rpc modes |
| misc (locales, etc.) | 5 MB | all |

No user benefits from both the JRE and the native binary at the same time — each `MODE` setting uses exactly one of them.

Additionally, the JRE pulls in ~280 MB of GUI libraries (LLVM, Mesa, GTK, icon themes) that signal-cli never uses. It is a headless CLI tool.

## Proposed Solution: Two Image Variants

| Variant | Tag pattern | Contains | Modes | Est. size |
|---|---|---|---|---|
| **JRE** | `latest`, `X.Y.Z` | Headless JRE + signal-cli Java dist | `normal`, `json-rpc` | **~660 MB** |
| **Native** | `latest-native`, `X.Y.Z-native` | signal-cli-native only (no JRE) | `native`, `json-rpc-native` | **~615 MB** |

For `arm/v7`, only the JRE variant is published (GraalVM doesn't produce a native binary for 32-bit ARM).

### Why the native variant (615 MB) isn't smaller than JRE (660 MB)

The 389 MB native binary includes all dependency code compiled as machine code plus an embedded runtime (GC, threading). It looks large next to the 121 MB Java dist, but that 121 MB is just compressed bytecode — it requires the separate 655 MB JRE to run. The fair comparison is:

- **Java stack**: 655 MB JRE + 121 MB jars = **776 MB total**
- **Native binary**: 389 MB (standalone, no JRE)

The native variant is genuinely lighter; its single large binary just happens to be larger than the `.jar` file viewed in isolation.

## Size Savings Breakdown

### JRE variant (~660 MB, down from 1.33 GB)

| Removal | Saved |
|---|---|
| signal-cli-native binary | -389 MB |
| Full JRE → headless JRE (drops LLVM, Mesa, GTK, icons) | -280 MB |
| **Total saved** | **-669 MB** |

### Native variant (~615 MB, down from 1.33 GB)

| Removal | Saved |
|---|---|
| Entire JRE stack | -655 MB |
| signal-cli Java dist (jars) | -121 MB |
| **Total saved** | **-776 MB** |
| **But adds back**: supervisor + python | +24 MB |

## Required Changes

### 1. `Dockerfile` — Split into multi-stage targets

The builder stage (lines 1–131) stays unchanged. The release stage (lines 133–194) becomes three stages: a shared `base`, then `jre` and `native` targets.

**Key changes:**

- `openjdk-25-jre` → `openjdk-25-jre-headless` (saves ~280 MB of GUI deps)
- JRE target: includes Java dist, excludes native binary
- Native target: includes native binary, excludes Java dist and JRE entirely
- Supervisor installed in both targets (needed for json-rpc daemon mode)

```dockerfile
# ---- Shared base for both variants ----
FROM ubuntu:noble AS base

ENV GIN_MODE=release
ENV PORT=8080

ARG SIGNAL_CLI_VERSION
ARG BUILD_VERSION_ARG

ENV BUILD_VERSION=$BUILD_VERSION_ARG
ENV SIGNAL_CLI_REST_API_PLUGIN_SHARED_OBJ_DIR=/usr/bin/

RUN dpkg-reconfigure debconf --frontend=noninteractive \
    && apt-get update \
    && apt-get install -y --no-install-recommends util-linux curl locales \
    && rm -rf /var/lib/apt/lists/*

COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/signal-cli-rest-api /usr/bin/signal-cli-rest-api
COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/scripts/jsonrpc2-helper /usr/bin/jsonrpc2-helper
COPY --from=buildcontainer /tmp/signal-cli-rest-api-src/signal-cli-rest-api_plugin_loader.so /usr/bin/signal-cli-rest-api_plugin_loader.so
COPY entrypoint.sh /entrypoint.sh

RUN userdel ubuntu -r \
    && groupadd -g 1000 signal-api \
    && useradd --no-log-init -M -d /home -s /bin/bash -u 1000 -g 1000 signal-api \
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

# ---- JRE variant: MODE=normal, json-rpc ----
FROM base AS jre

RUN dpkg-reconfigure debconf --frontend=noninteractive \
    && apt-get update \
    && apt-get install -y --no-install-recommends openjdk-25-jre-headless supervisor \
    && rm -rf /var/lib/apt/lists/*

COPY --from=buildcontainer /opt/signal-cli-${SIGNAL_CLI_VERSION} /opt/signal-cli-${SIGNAL_CLI_VERSION}

RUN ln -s /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli /usr/bin/signal-cli \
    && mkdir -p /home/.local/share/signal-cli

# ---- Native variant: MODE=native, json-rpc-native ----
FROM base AS native

RUN dpkg-reconfigure debconf --frontend=noninteractive \
    && apt-get update \
    && apt-get install -y --no-install-recommends supervisor \
    && rm -rf /var/lib/apt/lists/*

COPY --from=buildcontainer /tmp/signal-cli-native /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli-native

RUN mkdir -p /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin \
    && ln -s /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli-native /usr/bin/signal-cli-native \
    && mkdir -p /home/.local/share/signal-cli
```

### 2. `entrypoint.sh` — Add MODE validation

At the top of the entrypoint, before the existing logic, add validation that the requested MODE matches the available binaries in the image:

```sh
# Validate MODE against available binaries
if [ "$MODE" = "native" ] || [ "$MODE" = "json-rpc-native" ]; then
    if [ ! -x /usr/bin/signal-cli-native ]; then
        echo "ERROR: MODE=$MODE requires signal-cli-native, but this image doesn't include it."
        echo "Use the -native image tag (e.g., bbernhard/signal-cli-rest-api:latest-native)"
        exit 1
    fi
fi

if [ "$MODE" = "normal" ] || [ "$MODE" = "json-rpc" ]; then
    if ! command -v java >/dev/null 2>&1; then
        echo "ERROR: MODE=$MODE requires Java (signal-cli), but this image doesn't include it."
        echo "Use the standard image tag (e.g., bbernhard/signal-cli-rest-api:latest)"
        exit 1
    fi
fi
```

This gives a clear error instead of a cryptic failure when a user runs the wrong MODE for their image variant.

### 3. CI Workflows — Build both variants

All three workflow files need the same change: build two manifests using `--target`.

**`ci.yml`** — replaces the single `podman build` with two:

```yaml
- name: Build
  env:
    VERSION: ${{ github.run_number }}
  run: |
    df -h
    echo "Start CI build"
    docker run --privileged --rm tonistiigi/binfmt --install all

    podman manifest create build-jre
    podman build --format docker --target jre --platform linux/amd64,linux/arm64,linux/arm/v7 --manifest localhost/build-jre .
    podman manifest push localhost/build-jre docker://docker.io/bbernhard/signal-cli-rest-api:${EPOCHSECONDS}-ci

    podman manifest create build-native
    podman build --format docker --target native --platform linux/amd64,linux/arm64 --manifest localhost/build-native .
    podman manifest push localhost/build-native docker://docker.io/bbernhard/signal-cli-rest-api:${EPOCHSECONDS}-ci-native
```

**`release-productive-version.yml`** — same pattern, pushes tagged versions:

```yaml
- name: Release
  env:
    VERSION: ${{ github.event.inputs.version }}
  run: |
    echo "Start productive build"
    docker run --privileged --rm tonistiigi/binfmt --install all

    podman manifest create build-jre
    podman build --format docker --target jre --build-arg BUILD_VERSION_ARG=${VERSION} --platform linux/amd64,linux/arm64,linux/arm/v7 --manifest localhost/build-jre .
    podman manifest push localhost/build-jre docker://docker.io/bbernhard/signal-cli-rest-api:${VERSION}
    podman manifest push localhost/build-jre docker://docker.io/bbernhard/signal-cli-rest-api:latest

    podman manifest create build-native
    podman build --format docker --target native --build-arg BUILD_VERSION_ARG=${VERSION} --platform linux/amd64,linux/arm64 --manifest localhost/build-native .
    podman manifest push localhost/build-native docker://docker.io/bbernhard/signal-cli-rest-api:${VERSION}-native
    podman manifest push localhost/build-native docker://docker.io/bbernhard/signal-cli-rest-api:latest-native
```

**`release-dev-version.yml`** — same pattern, pushes `-dev` tags:

```yaml
    podman manifest push localhost/build-jre docker://docker.io/bbernhard/signal-cli-rest-api:${VERSION}-dev
    podman manifest push localhost/build-jre docker://docker.io/bbernhard/signal-cli-rest-api:latest-dev

    podman manifest push localhost/build-native docker://docker.io/bbernhard/signal-cli-rest-api:${VERSION}-dev-native
    podman manifest push localhost/build-native docker://docker.io/bbernhard/signal-cli-rest-api:latest-dev-native
```

### 4. `docker-compose.yml` — Add native variant example

```yaml
services:
  # JRE variant (default) — MODE=normal or MODE=json-rpc
  signal-cli-rest-api:
    image: bbernhard/signal-cli-rest-api:latest
    environment:
      - MODE=normal
    ports:
      - "8080:8080"
    volumes:
      - "./signal-cli-config:/home/.local/share/signal-cli"

  # Native variant — MODE=native or MODE=json-rpc-native
  # signal-cli-rest-api-native:
  #   image: bbernhard/signal-cli-rest-api:latest-native
  #   environment:
  #     - MODE=native
  #   ports:
  #     - "8080:8080"
  #   volumes:
  #     - "./signal-cli-config:/home/.local/share/signal-cli"
```

## Files That Do NOT Need Changes

| File | Why |
|---|---|
| `src/main.go` | The `SUPPORTS_NATIVE` env-var check and mode fallback still works. In the native image, `/usr/bin/signal-cli-native` exists; in the JRE image, it doesn't. The entrypoint validation catches mismatched MODE before the Go app starts. |
| `src/client/cli.go` | Already selects `signal-cli` or `signal-cli-native` based on mode. No awareness of image variants needed. |
| `src/scripts/jsonrpc2-helper.go` | Already selects the correct binary for daemon mode. No changes needed. |
| `publish.sh` | Just triggers GitHub Actions workflows. The workflow files handle variant tagging. |
| `.github/workflows/check-docs.yml` | Doc generation happens in the builder stage, unaffected by variant split. |
| `src/docs/*` | Auto-generated, no changes. |
| `plugins/*` | Runtime Lua scripts, no changes. |

## Tagging Scheme

| Tag | Variant | Platforms |
|---|---|---|
| `latest` | JRE | amd64, arm64, arm/v7 |
| `latest-native` | Native | amd64, arm64 |
| `X.Y.Z` | JRE | amd64, arm64, arm/v7 |
| `X.Y.Z-native` | Native | amd64, arm64 |
| `latest-dev` | JRE | amd64, arm64, arm/v7 |
| `latest-dev-native` | Native | amd64, arm64 |
| `X.Y.Z-dev` | JRE | amd64, arm64, arm/v7 |
| `X.Y.Z-dev-native` | Native | amd64, arm64 |

The `latest` tag points to the JRE variant (preserves backwards compatibility — default `MODE=normal` works out of the box).

## Backwards Compatibility

- **`latest` tag** continues to work with `MODE=normal` (the default) and `MODE=json-rpc`
- Users currently running `MODE=native` or `MODE=json-rpc-native` on the `latest` tag will get a clear error message from the new entrypoint validation, telling them to switch to `latest-native`
- For a transition period, the old all-in-one image could be published as `X.Y.Z-full` or `latest-full` if needed
- On arm/v7, only the JRE variant is published, matching current behavior (native mode falls back to normal)

## Quick Win (Without Splitting Images)

If splitting into two variants is too much CI complexity right now, a single-line change saves ~280 MB immediately:

```diff
- apt-get install -y --no-install-recommends util-linux supervisor openjdk-25-jre curl locales
+ apt-get install -y --no-install-recommends util-linux supervisor openjdk-25-jre-headless curl locales
```

This brings the image from **1.33 GB → ~1.05 GB** with zero functional impact, since signal-cli never uses AWT/Swing/GTK.