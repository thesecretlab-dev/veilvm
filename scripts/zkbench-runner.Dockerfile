FROM golang:1.23-bookworm

RUN apt-get update \
  && apt-get install -y --no-install-recommends gcc g++ make musl-tools ca-certificates git \
  && rm -rf /var/lib/apt/lists/*

ENV CGO_ENABLED=1

WORKDIR /workspace/examples/veilvm
