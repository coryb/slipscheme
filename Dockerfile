FROM scratch
MAINTAINER Cory Bennett <docker@corybennett.org> https://github.com/coryb/slipscheme
WORKDIR /work
COPY docker-root /
ENTRYPOINT ["/bin/slipscheme"]
ARG VERSION
ARG VCS_REF
LABEL org.label-schema.vcs-url=https://github.com/coryb/slipscheme \
      org.label-schema.vcs-ref=$VCS_REF \
      org.label-schema.version=$VERSION
