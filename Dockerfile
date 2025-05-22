FROM cgr.dev/chainguard/go:latest AS builder
ENV CGO_ENABLED=0
WORKDIR /workspace
COPY go.mod .
COPY go.sum .
COPY . .
RUN go mod download && go build .

FROM ghcr.io/anchore/syft:latest AS sbomgen
COPY --from=builder /workspace/vaban /usr/bin/vaban
RUN ["/syft", "--output", "spdx-json=/vaban.spdx.json", "/usr/bin/vaban"]

FROM cgr.dev/chainguard/static:latest
WORKDIR /tmp
COPY --from=builder /workspace/vaban /usr/bin/
COPY --from=sbomgen /vaban.spdx.json /var/lib/db/sbom/vaban.spdx.json
ENTRYPOINT ["/usr/bin/vaban"]
LABEL org.opencontainers.image.title="vaban"
LABEL org.opencontainers.image.description="Simple and Really Fast Varnish Cache Cluster Manager (for Varnish 6.x/7.x)"
LABEL org.opencontainers.image.url="https://github.com/tgragnato/vaban/"
LABEL org.opencontainers.image.source="https://github.com/tgragnato/vaban/"
LABEL license="MIT"
LABEL io.containers.autoupdate=registry
