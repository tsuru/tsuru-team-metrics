FROM golang:1.17.6-buster as builder

COPY . /src
WORKDIR /src
RUN go build -ldflags "-linkmode external -extldflags -static" -o /bin/tsuru-team-metrics

FROM scratch
COPY --from=builder /bin/tsuru-team-metrics /bin/tsuru-team-metrics
ENTRYPOINT ["/bin/tsuru-team-metrics"]
