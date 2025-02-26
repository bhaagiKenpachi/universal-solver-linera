# Build arguments:
#
# - `git_commit` is the hash of the current git commit — used for
#   versioning information inside the binaries.
# - `build_date` is the date and time of when the docker image was
#   built
# - `binaries` is the path to the directory containing the Linera
#   binaries. Leave unset to build the binaries from scratch.
# - `target` is a Rust target quadruple. Currently known to be
#   supported are `x86_64-unknown-linux-gnu` and
#   `aarch64-unknown-linux-gnu`.

# Stage 1 - Generate recipe file for dependencies

# Modern Docker provides TARGETPLATFORM and friends, but these do not
# provide enough information to reconstruct the whole Rust target and
# switching on them to map to a subset of targets seems unwise
ARG git_commit
ARG build_date
ARG target=x86_64-unknown-linux-gnu
ARG binaries=
ARG copy=${binaries:+_copy}

FROM rust:1.74-slim-bookworm AS builder
ARG git_commit
ARG target

RUN apt-get update && apt-get install -y \
    pkg-config \
    protobuf-compiler \
    clang