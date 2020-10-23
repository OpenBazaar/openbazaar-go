package wish

import (
	"reflect"
	"runtime"
	"strings"
)

func getCheckerShortName(fn Checker) string {
	fqn := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	cut := strings.LastIndex(fqn, ".")
	if cut < 0 {
		return fqn
	}
	return fqn[cut+1:]
}
