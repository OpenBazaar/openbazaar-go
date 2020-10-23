package commonatlases

import (
	"fmt"
	"time"

	"github.com/polydawn/refmt/json"
	"github.com/polydawn/refmt/obj/atlas"
)

func ExampleTime() {
	atl, _ := atlas.Build(Time_AsUnixInt)
	msg, _ := json.MarshalAtlased(json.EncodeOptions{}, time.Date(2014, 12, 25, 1, 0, 0, 0, time.UTC), atl)
	fmt.Printf("%s\n", msg)
	var t1 time.Time
	json.UnmarshalAtlased(msg, &t1, atl)
	fmt.Printf("%s\n", t1)

	atl, _ = atlas.Build(Time_AsRFC3339)
	msg, _ = json.MarshalAtlased(json.EncodeOptions{}, time.Date(2014, 12, 25, 1, 0, 0, 0, time.UTC), atl)
	fmt.Printf("%s\n", msg)
	var t2 time.Time
	json.UnmarshalAtlased(msg, &t2, atl)
	fmt.Printf("%s\n", t2)

	// Output:
	// 1419469200
	// 2014-12-25 01:00:00 +0000 UTC
	// "2014-12-25T01:00:00Z"
	// 2014-12-25 01:00:00 +0000 UTC
}
