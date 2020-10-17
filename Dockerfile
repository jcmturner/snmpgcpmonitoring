FROM golang:latest
WORKDIR /tmp
RUN go get -d -v github.com/jcmturner/snmpgcpmonitoring
RUN go build -a -o snmpcollect

FROM scratch
COPY files/targets.json /targets.json
COPY files/credentials.json /credentials.json
ENV TARGETS_CONF=/targets.json GCP_CREDENTIALS=/credentials.json
COPY --from=0 /tmp/snmpcollect /
CMD ["/snmpcollect"]