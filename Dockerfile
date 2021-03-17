FROM    golang:1.13-buster

WORKDIR /go/src/shuttle

COPY    dependencies .
RUN     cat dependencies | xargs go get -u

COPY    . .
RUN     go install .

EXPOSE  8000
CMD     ["shuttle"]
