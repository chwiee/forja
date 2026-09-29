FROM alpine:3.20
ARG VERSION=dev
RUN apk add --no-cache curl && echo "versao $VERSION" > /versao.txt
COPY hello.sh /usr/local/bin/hello.sh
RUN chmod +x /usr/local/bin/hello.sh
USER 1000
CMD ["/usr/local/bin/hello.sh"]
