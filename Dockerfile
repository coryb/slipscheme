FROM golang:alpine as build
COPY go.mod *.go /src/
WORKDIR /src
RUN CGO_ENABLED=0 go build

FROM scratch
COPY --from=build /src/slipscheme .
ENTRYPOINT ["./slipscheme"]
ARG VERSION
ARG VCS_REF
LABEL org.label-schema.vcs-url=https://github.com/coryb/slipscheme \
      org.label-schema.vcs-ref=$VCS_REF \
      org.label-schema.version=$VERSION
