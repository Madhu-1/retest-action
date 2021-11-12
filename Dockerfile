FROM golang:1.16

COPY . /home/src
WORKDIR /home/src
RUN go build -o /bin/action ./retest.go

ENTRYPOINT [ "/bin/action" ]
