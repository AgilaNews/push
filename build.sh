#!/usr/bin/env

DIR=$(dirname $0)
OUTPUT=$DIR/output
BIN=$OUTPUT/bin
APP_NAME=fcm_app_server

if [ -d $BIN ]
then
    mkdir -p $BIN
fi

go build -o $BIN/$APP_NAME -a  main/main.go
cp -r conf/* $OUTPUT/conf
