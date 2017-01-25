FROM ubuntu
MAINTAINER OpenBazaar Developers

RUN apt-get update && apt-get install -y ca-certificates

EXPOSE 4001
EXPOSE 4002
EXPOSE 8080

VOLUME /var/openbazaar

COPY ./dist/openbazaar-go-linux-amd64 /opt/openbazaard

CMD ["/opt/openbazaard", "start", "-t", "-d", "/var/openbazaar"]
