#!/bin/bash

RUST_LOG=info go test -count=1 ./... && cd rust && cargo test --release --all && cd ..
