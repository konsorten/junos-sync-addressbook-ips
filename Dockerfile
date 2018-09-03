FROM golang:1.10-alpine

COPY . /go/src/github.com/konsorten/junos-sync-pingdom-ips/

WORKDIR /go/src/github.com/konsorten/junos-sync-pingdom-ips/

RUN go get -v && go build && go install

ENTRYPOINT [ "/go/bin/junos-sync-pingdom-ips" ]
