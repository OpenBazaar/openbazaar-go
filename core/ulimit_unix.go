// +build darwin linux netbsd openbsd

package core

import (
	"fmt"
	"runtime"
	"syscall"
)

const fileDescriptorLimit uint64 = 32000

// CheckAndSetUlimit raises the file descriptor limit
func CheckAndSetUlimit() error {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return fmt.Errorf("error getting rlimit: %s", err)
	}

	var setting bool
	oldMax := rLimit.Max
	if rLimit.Cur < fileDescriptorLimit {
		if rLimit.Max < fileDescriptorLimit {
			rLimit.Max = fileDescriptorLimit
		}
		rLimit.Cur = fileDescriptorLimit
		setting = true
	}

	// If we're on darwin, work around the fact that Getrlimit reports
	// the wrong value. See https://github.com/golang/go/issues/30401
	if runtime.GOOS == "darwin" && rLimit.Cur > 10240 {
		// The max file limit is 10240, even though
		// the max returned by Getrlimit is 1<<63-1.
		// This is OPEN_MAX in sys/syslimits.h.
		rLimit.Max = 10240
		rLimit.Cur = 10240
	}

	// Try updating the limit. If it fails, try using the previous maximum instead
	// of our new maximum. Not all users have permissions to increase the maximum.
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		rLimit.Max = oldMax
		rLimit.Cur = oldMax
		err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
		if err != nil {
			return fmt.Errorf("error setting ulimit: %s", err)
		}
	}

	if !setting {
		log.Debug("Did not change ulimit")
		return nil
	}

	log.Debug("Successfully raised file descriptor limit to", rLimit.Cur)
	return nil
}
