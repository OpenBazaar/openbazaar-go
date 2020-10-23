package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

type BenchResults struct {
	Commit     CommitInfo  `json:"commit"`
	Tags       []string    `json:"tags"`
	Submitter  string      `json:"submitter"`
	Repo       string      `json:"repo"`
	Branch     string      `json:"branch"`
	RunDate    int64       `json:"runDate"`
	System     SysInfo     `json:"system"`
	Benchmarks []Benchmark `json:"benchmarks"`
}

type CommitInfo struct {
	Hash string `json:"hash"`
	Date int64  `json:"date"`
}

type SysInfo struct {
	CPU    string `json:"cpu"`
	OS     string `json:"os"`
	Memory string `json:"mem"`
}

type Benchmark struct {
	Name    string   `json:"name"`
	Module  string   `json:"module"`
	Metrics []Metric `json:"metrics"`
}

type Metric struct {
	Name   string  `json:"name"`
	Output bool    `json:"output"`
	Value  float64 `json:"value"`
	Type   string  `json:"type"`
}

func fatal(err error) {
	fmt.Println(err)
	os.Exit(1)
}

func getBasicInfo() (*BenchResults, error) {
	out := new(BenchResults)

	ci, err := getCommitInfo()
	if err != nil {
		return nil, err
	}

	out.Commit = *ci

	out.Submitter = os.Getenv("USER")

	branch, err := getBranch()
	if err != nil {
		return nil, err
	}
	out.Branch = branch

	sysinfo, err := getSystemInfo()
	if err != nil {
		return nil, err
	}
	out.System = *sysinfo

	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	out.Repo = filepath.Base(dir)

	out.RunDate = time.Now().Unix()

	return out, nil
}

func getCommitInfo() (*CommitInfo, error) {
	out, err := exec.Command("git", "show", "-s", "--no-color").Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	commitHash := strings.Fields(lines[0])[1]

	var dateStr string
	for _, l := range lines {
		if strings.HasPrefix(l, "Date:") {
			dateStr = strings.TrimSpace(l[5:])
			break
		}
	}

	t, err := time.Parse("Mon Jan 2 15:04:05 2006 -0700", dateStr)
	if err != nil {
		return nil, err
	}

	return &CommitInfo{
		Hash: commitHash,
		Date: t.Unix(),
	}, nil
}

func getBranch() (string, error) {
	out, err := exec.Command("git", "branch", "--no-color").Output()
	if err != nil {
		return "", err
	}

	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "*") {
			return strings.Trim(l, "* \t"), nil
		}
	}

	return "", fmt.Errorf("failed to determine git branch")
}

func getSystemInfo() (*SysInfo, error) {
	infostat, err := cpu.Info()
	if err != nil {
		return nil, err
	}

	vmem, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	return &SysInfo{
		CPU:    infostat[0].ModelName,
		OS:     runtime.GOOS,
		Memory: fmt.Sprint(vmem.Total),
	}, nil
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{".", "-bench=."}
	}

	var hasfilter bool
	for _, arg := range args {
		if strings.HasPrefix(arg, "-bench=") {
			hasfilter = true
			break
		}
	}

	if !hasfilter {
		args = append(args, "-bench=.")
	}

	args = append([]string{"test"}, args...)

	benchrun, err := getBasicInfo()
	if err != nil {
		fatal(err)
	}

	cmd := exec.Command("go", args...)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		fatal(err)
	}

	lines := strings.Split(string(out), "\n")
	var curpkg string
	for _, l := range lines {
		fields := strings.Fields(l)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "?", "PASS", "ok", "FAIL":
			continue
		case "pkg:":
			curpkg = fields[1]
		default:
			if !strings.HasPrefix(fields[0], "Benchmark") {
				continue
			}

			b := Benchmark{
				Name:   fields[0],
				Module: curpkg,
			}

			for i := 2; i < len(fields); i += 2 {
				val, err := strconv.ParseFloat(fields[i], 64)
				if err != nil {
					fmt.Printf("parsing line failed: %q\n", l)
					fatal(err)
				}

				metricName := fields[i+1]
				units := strings.Split(metricName, "/")[0]

				b.Metrics = append(b.Metrics, Metric{
					Name:   metricName,
					Value:  val,
					Output: true,
					Type:   units,
				})
			}
			benchrun.Benchmarks = append(benchrun.Benchmarks, b)
		}
	}

	jsonout, err := json.MarshalIndent(benchrun, "", "  ")
	if err != nil {
		fatal(err)
	}

	fmt.Println(string(jsonout))
}
