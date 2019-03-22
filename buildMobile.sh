#!/bin/bash

case "$TRAVIS_OS_NAME" in
  "linux")
  ;;

  "osx")

    brew install ant
    brew install gradle
    brew cask install homebrew/cask-versions/java8
    brew cask install android-sdk
    brew cask install android-ndk

    export ANT_HOME=/usr/local/opt/ant
    export MAVEN_HOME=/usr/local/opt/maven
    export GRADLE_HOME=/usr/local/opt/gradle
    export ANDROID_HOME=/usr/local/share/android-sdk
    export ANDROID_NDK_HOME=/usr/local/share/android-ndk

    export JAVA_OPTS='-XX:+IgnoreUnrecognizedVMOptions --add-modules java.se.ee'
    yes | sdkmanager "ndk-bundle" > /dev/null

    go get golang.org/x/mobile/cmd/gomobile

    gomobile init

    # Build iOS framework
    make ios_framework

    # Build Android framework
    make android_framework

  ;;
esac