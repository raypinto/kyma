# image builder base on golang:1.19.4-alpine3.17-v1
FROM eu.gcr.io/kyma-project/external/golang@sha256:a9b24b67dc83b3383d22a14941c2b2b2ca6a103d805cac6820fd1355943beaf1 as builder

ENV BASE_APP_DIR=/workspace/go/src/github.com/kyma-project/kyma/components/function-controller \
    CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=amd64 \
    LIBGIT2_VERSION=1.1.0-r2

RUN apk add --no-cache gcc libc-dev
RUN apk add --no-cache --repository http://dl-cdn.alpinelinux.org/alpine/v3.14/community libgit2-dev=${LIBGIT2_VERSION}

WORKDIR ${BASE_APP_DIR}

#
# copy files allowed in .dockerignore
#
COPY . ${BASE_APP_DIR}/

RUN go build -ldflags "-s -w" -a -o jobinit cmd/jobinit/main.go \
    && mkdir /app \
    && mv ./jobinit /app/jobinit

# result container base on alpine:3.17.2
FROM eu.gcr.io/kyma-project/external/alpine@sha256:e2e16842c9b54d985bf1ef9242a313f36b856181f188de21313820e177002501
ENV LIBGIT2_VERSION=1.1.0-r2

LABEL source = git@github.com:kyma-project/kyma.git

RUN apk update --no-cache && apk upgrade --no-cache
RUN apk add --no-cache ca-certificates
RUN apk add --no-cache --update --repository=http://dl-cdn.alpinelinux.org/alpine/edge/main openssh-client openssl
RUN apk add --no-cache --repository http://dl-cdn.alpinelinux.org/alpine/v3.14/community libgit2=${LIBGIT2_VERSION}

COPY --from=builder /app /app

ENTRYPOINT ["/app/jobinit"]
