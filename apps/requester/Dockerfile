FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/requester /usr/local/bin/requester

EXPOSE 9090
ENTRYPOINT ["/usr/local/bin/requester"]
