FROM golang:alpine AS builder
ARG o
ENV o=${o:-out}
RUN apk add --no-cache make git
WORKDIR /workspace
COPY . .
RUN make build && cp /workspace/${o} /workspace/main

FROM alpine AS final
COPY --from=builder /workspace/main ./
COPY pkg/certs certs

CMD [ "./main", "server", "--device-ip", "10.228.63.5:830", "--device-user", "onos", "--device-pass", "OnosDemo", "--insecure"]