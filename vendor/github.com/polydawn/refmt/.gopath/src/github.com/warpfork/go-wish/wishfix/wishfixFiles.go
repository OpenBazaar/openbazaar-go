package wishfix

import (
	"os"
)

func SaveFile(path string, hunks Hunks) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return MarshalHunks(f, hunks)
}

func LoadFile(path string) (*Hunks, error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	hunks, err := UnmarshalHunks(f)
	if err != nil {
		return nil, err
	}
	return hunks, nil
}

func MustSaveFile(path string, hunks Hunks) {
	if err := SaveFile(path, hunks); err != nil {
		panic(err)
	}
}

func MustLoadFile(path string) Hunks {
	hunks, err := LoadFile(path)
	if err != nil {
		panic(err)
	}
	return *hunks
}
