FROM golang:1.10-stretch AS builder

COPY . /go/src/github.com/konsorten/junos-sync-addressbook-ips/

WORKDIR /go/src/github.com/konsorten/junos-sync-addressbook-ips/

RUN go get && go build

FROM golang:1.10-stretch

ENV JUNIPER_HOST=
ENV JUNIPER_USER=root
ENV JUNIPER_PASSWORD=

COPY --from=builder /go/src/github.com/konsorten/junos-sync-addressbook-ips/junos-sync-addressbook-ips /go/bin/junos-sync-addressbook-ips

ENTRYPOINT [ "/go/bin/junos-sync-addressbook-ips" ]
