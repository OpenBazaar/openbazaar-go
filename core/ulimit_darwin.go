// +build darwin

package core

import (
	"fmt"
	"syscall"
)

const fileDescriptorLimit uint64 = 32000

func init() {
	err := checkAndSetUlimit()
	if err != nil {
		log.Error(err)
	}
}

func checkAndSetUlimit() error {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return fmt.Errorf("error getting rlimit: %s", err)
	}

	var setting bool
	if rLimit.Cur < fileDescriptorLimit {
		if rLimit.Max < fileDescriptorLimit {
			log.Error("adjusting max")
			rLimit.Max = fileDescriptorLimit
		}
		fmt.Printf("Adjusting current ulimit to %d...\n", fileDescriptorLimit)
		rLimit.Cur = fileDescriptorLimit
		setting = true
	}

	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return fmt.Errorf("error setting ulimit: %s", err)
	}

	if setting {
		fmt.Printf("Successfully raised file descriptor limit to %d.\n", fileDescriptorLimit)
	}

	return nil
}
