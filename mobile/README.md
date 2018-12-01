# openbazaar-go for mobile
## Purpose
This target allows a version of openbazaar-go to be compiled for use with Android or iOS. The primary changes are shorter timeouts around network requests and tighter resource management with a few details which allows a native frontend to speak with the server process.

## Prepare
There are a few dependencies which must be installed and setup before a build can be completed.

### iOS dependencies

- `go get golang.org/x/mobile/cmd/gomobile`
- `gomobile init`
- Install xcode and accept EULA/T&Cs.

### Android dependencies

- `go get -u golang.org/x/mobile/cmd/...`
- `gomobile init -ndk ~/Library/Android/sdk/ndk-bundle/` *(your Android NDK path may be different that this)*

## Build

### iOS 

- Execute `make ios_framework` in your local openbazaar-go repo. This should produce a `Mobile.framework` file which may be included in your iOS project.

### Android

- Execute `make android_framework` in your local openbazaar-go repo. These must be executed from the root of the project and cannot be built inside a virtualized container or process.
