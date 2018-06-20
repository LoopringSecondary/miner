FROM golang:1.9-alpine as builder

RUN apk add --no-cache make gcc musl-dev linux-headers
RUN mkdir /opt /opt/loopring /opt/loopring/miner /opt/loopring/miner/keystore /opt/loopring/miner/config /opt/loopring/miner/logs /opt/loopring/miner/logs/motan

ENV WORKSPACE=$GOPATH/src/github.com/Loopring/miner
ADD . $WORKSPACE

RUN cd $WORKSPACE && go build -ldflags -s -v  -o build/bin/miner cmd/lrc/*
RUN mv $WORKSPACE/build/bin/miner /$GOPATH/bin

ENTRYPOINT ["miner"]