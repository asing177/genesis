FROM golang:1.12.2-stretch as built

ADD . /go/src/github.com/Whiteblock/genesis

WORKDIR /go/src/github.com/Whiteblock/genesis
RUN go get && go build

FROM ubuntu:latest as final

RUN mkdir -p /genesis && apt-get update && apt-get install -y openssh-client ca-certificates
WORKDIR /genesis

COPY --from=built /go/src/github.com/Whiteblock/genesis/resources /genesis/resources
COPY --from=built /go/src/github.com/Whiteblock/genesis/config.json /genesis/config.json
COPY --from=built /go/src/github.com/Whiteblock/genesis/genesis /genesis/genesis

RUN ln -s /genesis/resources/geth/ /genesis/resources/ethereum

ENTRYPOINT ["/genesis/genesis"]