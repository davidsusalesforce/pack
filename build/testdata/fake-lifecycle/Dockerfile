FROM golang

RUN mkdir /lifecycle
WORKDIR /go/src/step
COPY . .
RUN GO111MODULE=on go build -mod=vendor -o /lifecycle/phase ./phase.go