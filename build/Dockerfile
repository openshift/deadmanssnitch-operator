FROM quay.io/redhat-services-prod/openshift/boilerplate:image-v6.0.1 AS builder
COPY . /go/src/github.com/openshift/deadmanssnitch-operator
WORKDIR /go/src/github.com/openshift/deadmanssnitch-operator
ENV GOFLAGS=""
RUN make go-build

FROM registry.access.redhat.com/ubi8/ubi-minimal:8.10-1295.1749680713
ENV OPERATOR_BIN=deadmanssnitch-operator

WORKDIR /root/
COPY --from=builder /go/src/github.com/openshift/deadmanssnitch-operator/build/_output/bin/${OPERATOR_BIN} /usr/local/bin/${OPERATOR_BIN}
LABEL io.openshift.managed.name="deadmanssnitch-operator" \
      io.openshift.managed.description="Operator to manage Dead Man Snitches"
