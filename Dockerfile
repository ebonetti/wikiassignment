FROM ebonetti/golang-petsc

#install and compile overpedia
ENV GO_DIR /usr/local/go
ENV GOPATH /go
ENV PATH $GOPATH/bin:$GO_DIR/bin:$PATH
ENV PROJECT github.com/ebonetti/wikiassignment
ADD . $GOPATH/src/$PROJECT
RUN go get $PROJECT/...;
WORKDIR /data