// +build windows

package files

import (
	"path/filepath"
	"strings"

	windows "gx/ipfs/QmVGjyM9i2msKvLXwh9VosCTgP4mL91kC7hDmqnwTTx6Hu/sys/windows"
)

func IsHidden(f File) bool {

	fName := filepath.Base(f.FileName())

	if strings.HasPrefix(fName, ".") && len(fName) > 1 {
		return true
	}

	p, e := windows.UTF16PtrFromString(f.FullPath())
	if e != nil {
		return false
	}

	attrs, e := windows.GetFileAttributes(p)
	if e != nil {
		return false
	}
	return attrs&windows.FILE_ATTRIBUTE_HIDDEN != 0
}
