MYPATH=$(shell pwd)
PROTOC=protoc
RM=rm
BIN=fcm_app_server
GOBUILD=go generate && go build -v && go test -v
SRCS=$(wildcard main/*.go db/*.go g/*.go service/*.go push/*.go device/*.go fcm/*.go gcm/*.go)
PBOBJS=$(patsubst %.proto,%.pb.go,$(wildcard ../comment/iface/*.proto))
OUTPUT=${MYPATH}/output
CONFDIR=${MYPATH}/conf
BINDIR=${MYPATH}/bin
RUNDIR=${MYPATH}/run

.PHONY: all

all: ${BIN}
	rm -rf ${OUTPUT}
	mkdir -p ${OUTPUT}/bin
	mkdir -p ${OUTPUT}/conf
	mkdir -p ${OUTPUT}/run
	cp ${BIN} ${OUTPUT}/bin
	cp ${BINDIR}/* ${OUTPUT}/bin
	cp ${CONFDIR}/* ${OUTPUT}/conf

clean:
	${RM} -f $(PBOBJS) ${BIN}

test:
	go test -v 

${BIN}: ${SRCS}
	go build -o $@ -v main/*.go

