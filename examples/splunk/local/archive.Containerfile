# The qualification helper supplies a static binary built from this repository.
FROM scratch
COPY archive-health /archive-health
USER 65532:65532
ENTRYPOINT ["/archive-health"]
