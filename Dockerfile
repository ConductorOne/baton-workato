FROM gcr.io/distroless/static-debian11:nonroot
ENTRYPOINT ["/baton-workato"]
COPY baton-workato /