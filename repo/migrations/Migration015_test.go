package migrations

import (
	"bytes"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func TestMigration015(t *testing.T) {

	// Test migration from old to new where the new directory does not exist.
	oldRepoPath0 := schema.GenerateTempPath()
	newRepoPath0 := schema.GenerateTempPath()
	if err := os.Mkdir(oldRepoPath0, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	testData := []byte("test")
	if err := ioutil.WriteFile(path.Join(oldRepoPath0, "testdata"), testData, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	migration := Migration015{ctx: schema.SchemaContext{
		DataPath:       oldRepoPath0,
		LegacyDataPath: oldRepoPath0,
		NewDataPath:    newRepoPath0,
	}}
	if err := migration.Up(oldRepoPath0, "", false); err != nil {
		t.Fatal(err)
	}

	readData, err := ioutil.ReadFile(path.Join(newRepoPath0, "testdata"))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(readData, testData) {
		t.Fatal("Failed to move old data directory into the correct location")
	}

	// Test migrating down when the old data directory no longer exists
	if err := os.RemoveAll(oldRepoPath0); err != nil {
		t.Fatal(err)
	}

	migration = Migration015{ctx: schema.SchemaContext{
		DataPath:       newRepoPath0,
		LegacyDataPath: oldRepoPath0,
		NewDataPath:    newRepoPath0,
	}}
	if err := migration.Down(newRepoPath0, "", false); err != nil {
		t.Fatal(err)
	}

	readData, err = ioutil.ReadFile(path.Join(oldRepoPath0, "testdata"))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(readData, testData) {
		t.Fatal("Failed to move new data directory back into the old location")
	}

	// Test migration from old to new where the new exists and gets moved back into place
	oldRepoPath1 := schema.GenerateTempPath()
	newRepoPath1 := schema.GenerateTempPath()
	if err := os.Mkdir(oldRepoPath1, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(newRepoPath1, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(oldRepoPath1)
	defer os.RemoveAll(newRepoPath1)

	if err := ioutil.WriteFile(path.Join(oldRepoPath1, "testdata"), testData, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	migration = Migration015{ctx: schema.SchemaContext{
		DataPath:       oldRepoPath1,
		LegacyDataPath: oldRepoPath1,
		NewDataPath:    newRepoPath1,
	}}
	if err := migration.Up(oldRepoPath1, "", false); err != nil {
		t.Fatal(err)
	}

	readData, err = ioutil.ReadFile(path.Join(newRepoPath1, "testdata"))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(readData, testData) {
		t.Fatal("Failed to move old data directory into the correct location")
	}

	if _, err := os.Stat(newRepoPath1 + ".old"); os.IsNotExist(err) {
		t.Error("Failed to move directory in the new location a .old location")
	}

	// Test migrating down when the old data directory still exits
	if err := os.Mkdir(oldRepoPath1, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	migration = Migration015{ctx: schema.SchemaContext{
		DataPath:       newRepoPath1,
		LegacyDataPath: oldRepoPath1,
		NewDataPath:    newRepoPath1,
	}}
	if err := migration.Down(newRepoPath1, "", false); err != nil {
		t.Fatal(err)
	}

	readData, err = ioutil.ReadFile(path.Join(oldRepoPath1, "testdata"))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(readData, testData) {
		t.Fatal("Failed to move new data directory back into the old location")
	}

	if _, err := os.Stat(oldRepoPath1 + ".old"); os.IsNotExist(err) {
		t.Error("Failed to move directory in the new location a .old location")
	}
}
