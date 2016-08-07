FROM scratch
MAINTAINER Cory Bennett <docker@corybennett.org> https://github.com/coryb/slipscheme
WORKDIR /work
COPY docker-root /
ENTRYPOINT ["/bin/slipscheme"]
