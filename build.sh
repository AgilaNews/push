#!/usr/bin/env

DIR=$(dirname $0)
OUTPUT=$DIR/output
BIN=$OUTPUT/bin
APP_NAME=fcm_app_server
CONF=$OUTPUT/conf

rm -rf $OUTPUT
if [ ! -d $BIN ]
then
    mkdir -p $BIN
fi
if [ ! -d $CONF ]
then
    mkdir -p $CONF
fi

go build -o $BIN/$APP_NAME -a  main/main.go
cp conf/* $CONF/
