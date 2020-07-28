#!/bin/bash

set -e

make clean
cd rust
rm Cargo.lock
cargo update -p "filecoin-proofs-api"
cargo install cbindgen
cbindgen --clean --config cbindgen.toml --crate filcrypto --output ../include/filcrypto.h
cd ..
FFI_BUILD_FROM_SOURCE=1 make
