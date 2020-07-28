package wish

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	//	"github.com/warpfork/go-wish/cmp"
)

func execGoTest(t *testing.T, testName string, verbose string) string {
	if os.Getenv("forked") != "" {
		panic("forkbomb avoidance")
	}

	cmd := exec.Command("go", "test", "-run", testName, verbose)
	cmd.Env = append(os.Environ(), "forked=1")
	nom, err := cmd.CombinedOutput()
	if len(nom) == 0 { // lack of output means a real bug.  nonzero exits are fine, here.
		t.Fatalf("exec failed: %s", err)
	}

	return denumerator.Replace(string(nom))
}

// we need this to strip out timing info and excessive line numbers from tests
var denumerator = strings.NewReplacer(
	"0", "N",
	"1", "N",
	"2", "N",
	"3", "N",
	"4", "N",
	"5", "N",
	"6", "N",
	"7", "N",
	"8", "N",
	"9", "N",
)

func TestGoTestOutputTree_helper(t *testing.T) {
	if os.Getenv("forked") == "" {
		t.SkipNow()
	}
	t.Run("subtest", func(t *testing.T) {
		t.Logf("wtf")
		t.Run("subsubtest", func(t *testing.T) {
			t.Errorf("sadz")
		})
		t.Run("happy subsubtest", func(t *testing.T) {
			t.Logf("ooh!\n")
		})
	})
}

func TestGoTestOutputTree(t *testing.T) {
	t.Run("non-verbose", func(t *testing.T) {
		nom := execGoTest(t, "TestGoTestOutputTree_helper", "")
		diff := strdiff(nom, Dedent(`
			--- FAIL: TestGoTestOutputTree_helper (N.NNs)
			    --- FAIL: TestGoTestOutputTree_helper/subtest (N.NNs)
			        output_test.go:NN: wtf
			        --- FAIL: TestGoTestOutputTree_helper/subtest/subsubtest (N.NNs)
			            output_test.go:NN: sadz
			FAIL
			FAIL	github.com/warpfork/go-wish	N.NNNs
		`))
		if diff != "" {
			t.Errorf("%s", diff)
		}
	})
	t.Run("verbose", func(t *testing.T) {
		nom := execGoTest(t, "TestGoTestOutputTree_helper", "-v")
		diff := strdiff(nom, Dedent(`
			=== RUN   TestGoTestOutputTree_helper
			=== RUN   TestGoTestOutputTree_helper/subtest
			=== RUN   TestGoTestOutputTree_helper/subtest/subsubtest
			=== RUN   TestGoTestOutputTree_helper/subtest/happy_subsubtest
			--- FAIL: TestGoTestOutputTree_helper (N.NNs)
			    --- FAIL: TestGoTestOutputTree_helper/subtest (N.NNs)
			        output_test.go:NN: wtf
			        --- FAIL: TestGoTestOutputTree_helper/subtest/subsubtest (N.NNs)
			            output_test.go:NN: sadz
			        --- PASS: TestGoTestOutputTree_helper/subtest/happy_subsubtest (N.NNs)
			            output_test.go:NN: ooh!
			FAIL
			exit status N
			FAIL	github.com/warpfork/go-wish	N.NNNs
		`))
		if diff != "" {
			t.Errorf("%s", diff)
		}
	})
}

func TestGoTestOutputFun_helper(t *testing.T) {
	if os.Getenv("forked") == "" {
		t.SkipNow()
	}
	t.Run("subtest", func(t *testing.T) {
		t.Logf("hello!")
		t.Run("subsubtest", func(t *testing.T) {
			Wish(t, "snafoo", ShouldEqual, "zounds")
			Wish(t, "zebras", ShouldEqual, "cats")
			Wish(t, struct{ Foo string }{}, ShouldEqual, struct{ Bar string }{})
			Wish(t, "orange", ShouldEqual, "orange")
		})
		t.Run("happy subsubtest", func(t *testing.T) {
			Wish(t, "orange", ShouldEqual, "orange")
		})
	})
}

func TestGoTestOutputFun(t *testing.T) {
	t.Run("non-verbose", func(t *testing.T) {
		nom := execGoTest(t, "TestGoTestOutputFun_helper", "")
		diff := strdiff(nom, Dedent(`
			--- FAIL: TestGoTestOutputFun_helper (N.NNs)
			    --- FAIL: TestGoTestOutputFun_helper/subtest (N.NNs)
			        output_test.go:NNN: hello!
			        --- FAIL: TestGoTestOutputFun_helper/subtest/subsubtest (N.NNs)
			            output_test.go:NNN: ShouldEqual check rejected:
			                	@@ -N +N @@
			                	- snafoo
			                	+ zounds
			                
			            output_test.go:NNN: ShouldEqual check rejected:
			                	@@ -N +N @@
			                	- zebras
			                	+ cats
			                
			            output_test.go:NNN: ShouldEqual check rejected:
			                	  interface{}(
			                	- 	struct{ Foo string }{},
			                	+ 	struct{ Bar string }{},
			                	  )
			                
			FAIL
			FAIL	github.com/warpfork/go-wish	N.NNNs
		`))
		if diff != "" {
			t.Errorf("%s", diff)
		}
	})
	t.Run("verbose", func(t *testing.T) {
		nom := execGoTest(t, "TestGoTestOutputFun_helper", "-v")
		diff := strdiff(nom, Dedent(`
			=== RUN   TestGoTestOutputFun_helper
			=== RUN   TestGoTestOutputFun_helper/subtest
			=== RUN   TestGoTestOutputFun_helper/subtest/subsubtest
			=== RUN   TestGoTestOutputFun_helper/subtest/happy_subsubtest
			--- FAIL: TestGoTestOutputFun_helper (N.NNs)
			    --- FAIL: TestGoTestOutputFun_helper/subtest (N.NNs)
			        output_test.go:NNN: hello!
			        --- FAIL: TestGoTestOutputFun_helper/subtest/subsubtest (N.NNs)
			            output_test.go:NNN: ShouldEqual check rejected:
			                	@@ -N +N @@
			                	- snafoo
			                	+ zounds
			                
			            output_test.go:NNN: ShouldEqual check rejected:
			                	@@ -N +N @@
			                	- zebras
			                	+ cats
			                
			            output_test.go:NNN: ShouldEqual check rejected:
			                	  interface{}(
			                	- 	struct{ Foo string }{},
			                	+ 	struct{ Bar string }{},
			                	  )
			                
			        --- PASS: TestGoTestOutputFun_helper/subtest/happy_subsubtest (N.NNs)
			FAIL
			exit status N
			FAIL	github.com/warpfork/go-wish	N.NNNs
		`))
		if diff != "" {
			t.Errorf("%s", diff)
		}
	})
}
