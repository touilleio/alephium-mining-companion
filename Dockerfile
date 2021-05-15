FROM gcr.io/distroless/base

COPY alephium-mining-sidecar /alephium-mining-sidecar

USER nobody

ENTRYPOINT ["/alephium-mining-sidecar"]
EXPOSE 8080

HEALTHCHECK --interval=60s --timeout=10s --retries=1 --start-period=30s CMD ["/alephium-mining-sidecar", "--health-check"]
