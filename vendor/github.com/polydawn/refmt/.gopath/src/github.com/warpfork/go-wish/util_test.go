package wish

import "testing"

func TestGetCheckerShortName(t *testing.T) {
	sn := getCheckerShortName(ShouldBe)
	if sn != "ShouldBe" {
		t.Errorf("%q", sn)
	}
}
