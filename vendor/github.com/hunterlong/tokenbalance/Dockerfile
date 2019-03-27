FROM golang:1.11-alpine as base
MAINTAINER "Hunter Long (https://github.com/hunterlong)"
ARG VERSION
ENV DEP_VERSION v0.5.0
RUN apk add --no-cache libstdc++ gcc g++ make git ca-certificates linux-headers curl
RUN curl -L -s https://github.com/golang/dep/releases/download/$DEP_VERSION/dep-linux-amd64 -o /go/bin/dep && \
    chmod +x /go/bin/dep
WORKDIR /go/src/github.com/hunterlong/tokenbalance
ADD . /go/src/github.com/hunterlong/tokenbalance
RUN make dep
RUN make static

# TokenBalance :latest Docker Image
FROM alpine:latest
MAINTAINER "Hunter Long (https://github.com/hunterlong)"
ARG VERSION
RUN apk --no-cache add libstdc++ ca-certificates curl jq

# make static
COPY --from=base /go/src/github.com/hunterlong/tokenbalance/tokenbalance /usr/local/bin/tokenbalance

WORKDIR /app
ENV GETH_SERVER https://eth.coinapp.io
ENV PORT 8080
EXPOSE $PORT
HEALTHCHECK --interval=30s --timeout=10s --retries=5 CMD curl -s "http://localhost:$PORT/health" | jq -r -e ".online==true"

ENTRYPOINT tokenbalance start --geth $GETH_SERVER --port $PORT --ip 0.0.0.0