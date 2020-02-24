package factory

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

const defaultProfileFixture = "v5-profile-moderator-fixed-fee"

func MustLoadProfileFixture(fixtureName string) []byte {
	filename := filepath.Join(fixtureLoadPath(), "profiles", fmt.Sprintf("%s.json", fixtureName))
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(fmt.Sprintf("cannot find fixture (%s): %s", filename, err))
	}
	return b
}

func MustNewProfile() *repo.Profile {
	p, err := repo.NewProfileFromProtobuf(MustNewProfileProtobuf())
	if err != nil {
		panic(err.Error())
	}
	return p
}

func MustNewProfileProtobuf() *pb.Profile {
	var (
		p   = new(pb.Profile)
		err = jsonpb.UnmarshalString(string(MustLoadProfileFixture(defaultProfileFixture)), p)
	)
	if err != nil {
		panic(err.Error())
	}
	return p
}
