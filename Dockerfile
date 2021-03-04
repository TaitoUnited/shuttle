FROM    golang:1.13-buster

WORKDIR /go/src/shuttle

COPY    . .

RUN     cat dependencies | xargs go get -u
RUN     go install .

EXPOSE  8000
CMD     ["shuttle"]
