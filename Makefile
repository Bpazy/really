BINDIR=bin
VERSION=$(shell git describe --tags || echo "unknownversion")
ifdef NAME
	FULLNAME=$(NAME)
else
	FULLNAME=behappy-$@-$(VERSION)
endif
LDFLAGS="-s -w -X github.com/Bpazy/behappy.buildVer=${VERSION}"
GOBUILD=go build -ldflags=${LDFLAGS}
CMDPATH=./cmd/behappy
export GOPROXY=https://mirrors.aliyun.com/goproxy/

all: linux-amd64 darwin-amd64 windows-amd64 # Most used

darwin-amd64:
	GOARCH=amd64 GOOS=darwin $(GOBUILD) -o $(BINDIR)/$(FULLNAME) $(CMDPATH)

linux-amd64:
	GOARCH=amd64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(FULLNAME) $(CMDPATH)

windows-amd64:
	GOARCH=amd64 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(FULLNAME).exe $(CMDPATH)

install:
	go install -ldflags=${LDFLAGS} $(CMDPATH)

clean:
	rm $(BINDIR)/*
