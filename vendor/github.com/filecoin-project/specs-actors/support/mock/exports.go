package mock

import (
	"fmt"
	"reflect"
	"testing"
)

func CheckActorExports(t *testing.T, act interface{ Exports() []interface{} }) {
	for i, m := range act.Exports() {
		if i == 0 { // Send is implicit
			continue
		}

		if m == nil {
			continue
		}

		t.Run(fmt.Sprintf("metdod%d", i), func(t *testing.T) {
			mrt := Runtime{t: t}
			mrt.verifyExportedMethodType(reflect.ValueOf(m))
		})
	}
}
