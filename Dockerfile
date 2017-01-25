FROM ubuntu
MAINTAINER OpenBazaar Developers

COPY ./dist/openbazaar-go-linux-amd64 /

EXPOSE 4001
EXPOSE 4002
EXPOSE 8080

VOLUME /data

CMD ["/openbazaar-go-linux-amd64", "start", "-d", "/data", "-t"]
