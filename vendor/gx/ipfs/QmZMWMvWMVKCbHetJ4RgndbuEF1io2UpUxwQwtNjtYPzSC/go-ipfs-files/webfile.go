package files

import (
	"io"
	"net/http"
	"net/url"
	"path/filepath"
)

// WebFile is an implementation of File which reads it
// from a Web URL (http). A GET request will be performed
// against the source when calling Read().
type WebFile struct {
	body io.ReadCloser
	url  *url.URL
}

// NewWebFile creates a WebFile with the given URL, which
// will be used to perform the GET request on Read().
func NewWebFile(url *url.URL) *WebFile {
	return &WebFile{
		url: url,
	}
}

// Read reads the File from it's web location. On the first
// call to Read, a GET request will be performed against the
// WebFile's URL, using Go's default HTTP client. Any further
// reads will keep reading from the HTTP Request body.
func (wf *WebFile) Read(b []byte) (int, error) {
	if wf.body == nil {
		resp, err := http.Get(wf.url.String())
		if err != nil {
			return 0, err
		}
		wf.body = resp.Body
	}
	return wf.body.Read(b)
}

// Close closes the WebFile (or the request body).
func (wf *WebFile) Close() error {
	if wf.body == nil {
		return nil
	}
	return wf.body.Close()
}

// FullPath returns the "Host+Path" for this WebFile.
func (wf *WebFile) FullPath() string {
	return wf.url.Host + wf.url.Path
}

// FileName returns the last element of the URL
// path for this file.
func (wf *WebFile) FileName() string {
	return filepath.Base(wf.url.Path)
}

// IsDirectory returns false.
func (wf *WebFile) IsDirectory() bool {
	return false
}

// NextFile always returns an ErrNotDirectory error.
func (wf *WebFile) NextFile() (File, error) {
	return nil, ErrNotDirectory
}
