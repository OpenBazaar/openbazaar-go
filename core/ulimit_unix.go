// +build darwin linux netbsd openbsd

package core

import (
	"fmt"
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

	log.Debug("Successfully raised file descriptor limit to", fileDescriptorLimit)
	return nil
}
