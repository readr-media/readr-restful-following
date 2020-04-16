FROM golang:1.14-alpine

WORKDIR /
RUN export GIN_MODE=release

ADD app /app

ADD config /config

VOLUME /var/log
EXPOSE 8080

CMD ["./app"]
