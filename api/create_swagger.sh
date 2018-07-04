#!/bin/bash

swagger generate spec -b . -o ./swagger.json --scan-models
swagger flatten swagger.json > flatten.json
swagger serve -F=swagger flatten.json
