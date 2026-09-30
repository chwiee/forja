FROM alpine:3.20
RUN apk add --no-cache file && echo "buildado em $(uname -m)" > /build-arch.txt
CMD ["sh", "-c", "echo \"rodando em $(uname -m) | $(cat /build-arch.txt) | $(file -b /bin/busybox | cut -d, -f2)\""]
