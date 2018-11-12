#!/bin/sh

(
    cd "js"
    npm install
)

(
    cd "js" && npm start
) &

sleep 1

(
    cd "go" && go run main.go
) &

wait
