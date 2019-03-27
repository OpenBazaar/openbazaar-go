// +build windows

package files

import (
	"path/filepath"
	"strings"

	windows "gx/ipfs/QmVGjyM9i2msKvLXwh9VosCTgP4mL91kC7hDmqnwTTx6Hu/sys/windows"
)

func IsHidden(name string, f Node) bool {

	fName := filepath.Base(name)

	if strings.HasPrefix(fName, ".") && len(fName) > 1 {
		return true
	}

	fi, ok := f.(FileInfo)
	if !ok {
		return false
	}

	p, e := windows.UTF16PtrFromString(fi.AbsPath())
	if e != nil {
		return false
	}

	attrs, e := windows.GetFileAttributes(p)
	if e != nil {
		return false
	}
	return attrs&windows.FILE_ATTRIBUTE_HIDDEN != 0
}
