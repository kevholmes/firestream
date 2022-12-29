FROM golang:1.15 AS builder

WORKDIR /srv/firestream/code
COPY . .
RUN go build -ldflags "-X main.appBuildTime=`date -u '+%Y-%m-%d_%I:%M:%S%p'` -X main.appGitHash=`git rev-parse HEAD`" \
    && rm -rf .git* && mkdir ../bin && mv firestream ../bin && cd .. ; rm -rf code/ ;

FROM centos:7
WORKDIR /srv/firestream
COPY --from=builder /srv/firestream/bin/ .
# no port yet for firestream health etc.
#EXPOSE 8030
RUN adduser -u 4600 goservice
USER goservice
CMD /srv/firestream/firestream
