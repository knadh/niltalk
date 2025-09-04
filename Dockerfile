FROM alpine:latest AS deploy
WORKDIR /niltalk
COPY niltalk .
COPY config.sample.toml config.toml
ENTRYPOINT [ "./niltalk" ]
