FROM golang:1.10-stretch AS builder

COPY . /go/src/github.com/konsorten/junos-sync-pingdom-ips/

WORKDIR /go/src/github.com/konsorten/junos-sync-pingdom-ips/

RUN go get && go build

FROM golang:1.10-stretch

ENV JUNIPER_HOST=
ENV JUNIPER_USER=root
ENV JUNIPER_PASSWORD=

COPY --from=builder /go/src/github.com/konsorten/junos-sync-pingdom-ips/junos-sync-pingdom-ips /go/bin/junos-sync-pingdom-ips

ENTRYPOINT [ "/go/bin/junos-sync-pingdom-ips" ]
