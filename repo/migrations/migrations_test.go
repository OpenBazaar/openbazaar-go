package migrations_test

import (
	"io/ioutil"
	"testing"
)

func assertCorrectRepoVer(t *testing.T, verPath, expectedRepoVer string) {
	repoVer, err := ioutil.ReadFile(verPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(repoVer) != expectedRepoVer {
		t.Fatal("Failed to write new repo version")
	}
}

func assertCorrectFileContents(t *testing.T, filePath string, expectedFileContents string) {
	fileContents, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(fileContents) != expectedFileContents {
		t.Fatal("Incorrect file content:", filePath)
	}
}
