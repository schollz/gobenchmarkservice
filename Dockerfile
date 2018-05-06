# Build with:
# 	docker build -t benchservice .
# Run with
# 	docker run --name gobenchservice -e "benchredis=192.168.50.235:6374" -t benchservice
#   docker start/stop gobenchservice
FROM golang:1.10-alpine3.7
RUN echo "version1" && apk add --no-cache git mercurial \
	&& go get -v github.com/schollz/git \
	&& go install -v github.com/schollz/git
ENV PATH "$GOPATH/bin:$PATH"
RUN go get -v github.com/schollz/gobenchmarkservice \
	&& go install -v github.com/schollz/gobenchmarkservice
ENTRYPOINT gobenchmarkservice -redis $benchredis -client