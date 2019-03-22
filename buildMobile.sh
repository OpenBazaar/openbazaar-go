#!/bin/bash

case "$TRAVIS_OS_NAME" in
  "linux")
  ;;

  "osx")

    brew install ant
    brew install maven
    brew install gradle
    brew install android-sdk
    brew install android-ndk

    android update sdk --no-ui

    export ANT_HOME=/usr/local/opt/ant
    export MAVEN_HOME=/usr/local/opt/maven
    export GRADLE_HOME=/usr/local/opt/gradle
    export ANDROID_HOME=/usr/local/opt/android-sdk
    export ANDROID_NDK_HOME=/usr/local/opt/android-ndk

    

    # Build iOS framework
    make ios_framework

    # Build Android framework
    make android_framework

  ;;
esac