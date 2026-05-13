FROM gcr.io/distroless/static-debian12:nonroot

COPY loglens /usr/local/bin/loglens

USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/loglens"]
