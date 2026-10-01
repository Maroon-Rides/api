FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/gateway /usr/local/bin/gateway

EXPOSE 3000
ENTRYPOINT ["/usr/local/bin/gateway"]
