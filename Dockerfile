FROM alpine:latest AS deploy
WORKDIR /niltalk
COPY niltalk .
COPY config.toml.sample config.toml
ENTRYPOINT [ "./niltalk" ]
