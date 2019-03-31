FROM arm32v6/golang:1.11-alpine AS builder1

# Install packages
RUN apk add --update --no-cache git make \
    && rm -rf /var/cache/apk/* \
    && go get -u github.com/golang/dep/cmd/dep

# Compile application
WORKDIR /go/src/github.com/mysteriumnetwork/node
COPY . .
RUN make dep-ensure

FROM arm32v6/golang:1.11-alpine AS builder2

# Install packages
RUN apk add --update --no-cache bash gcc musl-dev make \
    && rm -rf /var/cache/apk/*

ARG BUILD_BRANCH=$BUILD_BRANCH
ARG BUILD_COMMIT=$BUILD_COMMIT
ARG BUILD_NUMBER=$BUILD_NUMBER
ARG BUILD_VERSION=$BUILD_VERSION

# Compile application
WORKDIR /go/src/github.com/mysteriumnetwork/node
COPY . .
COPY --from=builder1 /go/src/github.com/mysteriumnetwork/node/vendor ./vendor
RUN make build

FROM arm32v6/alpine:3.9 AS runner

# Install packages
RUN apk add --update --no-cache iptables ca-certificates openvpn bash sudo \
    && rm -rf /var/cache/apk/*

COPY bin/helpers/prepare-run-env.sh /usr/local/bin/prepare-run-env.sh
COPY bin/docker/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

COPY bin/package/config/common /etc/mysterium-node
COPY bin/package/config/linux /etc/mysterium-node

COPY --from=builder2 /go/src/github.com/mysteriumnetwork/node/build/myst/myst /usr/bin/myst

WORKDIR /var/run/myst
