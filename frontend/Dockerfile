# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Build stage
#
# The Flutter SDK is fetched from the official release archive rather than taken
# from a published image. The reason is concrete: the newest cirruslabs image
# carries Dart 3.12, this project requires ^3.13.4, and no published tag
# satisfies that — `:stable` is built on a delay and `flutter pub get` fails on
# version solving before a single line of Dart is compiled.
#
# Upgrading means changing both ARG values together. The checksum is taken from
# releases_linux.json in the flutter_infra_release bucket, and pinning it is the
# point: without it, an archive that changed underneath would be built silently.
# ---------------------------------------------------------------------------
# The platform is pinned to amd64 on purpose. Flutter ships no arm64 Linux SDK —
# the release manifest lists exactly one Linux archive and it is x86-64 — so on
# an Apple Silicon host the dart binary would run inside an arm64 container that
# has no x86 loader, and it dies with "rosetta error: failed to open elf at
# /lib64/ld-linux-x86-64.so.2". Declaring the platform gives the stage a complete
# x86 sysroot to be emulated into: slow, but correct. On an Intel host this line
# is a no-op, so the same file works on both.
FROM --platform=linux/amd64 debian:bookworm-slim AS builder

# Where the browser looks for the API. Overridable at build time, because it is
# a property of the deployment and not of the source:
#   docker build --build-arg API_BASE_URL=http://192.168.1.10:8080 .
ARG API_BASE_URL=http://localhost:8080

ARG FLUTTER_VERSION=3.47.5
ARG FLUTTER_SHA256=2132e990f236f8d22e7c6314b29a191a95b10d7cbcfec9b4e2e303d996652cbb

ENV FLUTTER_ROOT=/opt/flutter
ENV PATH="${FLUTTER_ROOT}/bin:${PATH}"
ENV FLUTTER_SUPPRESS_ANALYTICS=true
ENV PUB_CACHE=/pub-cache

# ca-certificates and curl for the download, git because the tool derives its own
# version from the SDK's repository, xz-utils and unzip to unpack the archive and
# the engine artifacts it then downloads.
RUN apt-get update \
    && apt-get install --no-install-recommends -y \
        ca-certificates curl git unzip xz-utils \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL -o /tmp/flutter.tar.xz \
        "https://storage.googleapis.com/flutter_infra_release/releases/stable/linux/flutter_linux_${FLUTTER_VERSION}-stable.tar.xz" \
    && echo "${FLUTTER_SHA256}  /tmp/flutter.tar.xz" | sha256sum -c - \
    && mkdir -p "${FLUTTER_ROOT}" \
    && tar -xJf /tmp/flutter.tar.xz -C "${FLUTTER_ROOT}" --strip-components=1 \
    && rm /tmp/flutter.tar.xz

# The archive preserves the UID of whoever built it, so git inside the SDK sees
# an owner that is not the one running this build and refuses to read it
# ("detected dubious ownership"). Flutter derives its own version by running
# git there, so the exception is required rather than cosmetic. A chown would
# work too, but it walks two and a half gigabytes for no benefit.
RUN git config --global --add safe.directory "${FLUTTER_ROOT}"

# Without this the tool tries to update itself and the analytics prompt, and the
# first build reaches for the network twice for no reason.
RUN flutter config --no-analytics \
    && flutter --version

WORKDIR /src

# Dependencies in their own layer. A change under lib/ then rebuilds the bundle
# without re-resolving the dependency graph.
COPY pubspec.yaml pubspec.lock ./
RUN flutter pub get

COPY . .

# Release mode, not debug: a debug web build fetches the Dart VM from
# gstatic.com at page load and runs several times slower, which is exactly the
# build nobody should measure performance with.
#
# --dart-define values are compiled in, so changing the API address means a
# rebuild — there is no way to repoint a served bundle at runtime.
RUN flutter build web \
        --release \
        --dart-define=API_BASE_URL=${API_BASE_URL}

# ---------------------------------------------------------------------------
# Final stage
#
# Only nginx and the static output. The SDK, the pub cache and the Dart kernel
# snapshots stay in the build stage, so the shipped image is tens of megabytes
# instead of gigabytes.
# ---------------------------------------------------------------------------
FROM nginx:alpine

COPY --from=builder /src/build/web /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

# nginx serves the app shell immediately, so this checks the bundle is actually
# reachable rather than that the process is alive.
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://localhost:80/ || exit 1