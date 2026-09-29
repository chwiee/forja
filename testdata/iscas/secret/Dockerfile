FROM docker.io/library/alpine:3.22
ENV DB_PASSWORD=senha-de-laboratorio
ARG API_TOKEN
RUN echo "token=$API_TOKEN" > /etc/app.conf
RUN adduser -D app
USER app
