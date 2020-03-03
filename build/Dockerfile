FROM golang:alpine AS builder
ARG o
ENV o=${o:-out}
RUN apk add --no-cache make git
WORKDIR /workspace
COPY . .
RUN make build && cp /workspace/${o} /workspace/main

FROM alpine AS final
COPY --from=builder /workspace/main ./
CMD [ "./main", "server"]