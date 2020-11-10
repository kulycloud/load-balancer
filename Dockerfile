FROM golang:1.15.3-alpine AS builder

ADD load-balancer/go.mod load-balancer/go.sum /build/load-balancer/
ADD protocol/go.mod protocol/go.sum /build/protocol/
ADD common/go.mod common/go.sum /build/common/

ENV CGO_ENABLED=0

WORKDIR /build/load-balancer
RUN go mod download

COPY load-balancer/ /build/load-balancer/
COPY protocol/ /build/protocol
COPY common/ /build/common
RUN go build -o /build/kuly .

FROM scratch

COPY --from=builder /build/kuly /

CMD ["/kuly"]
