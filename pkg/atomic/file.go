package atomic

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// WriteAndCopy will write the provided data to disk in a temp
// file, and then atomically copy that to the provided path.
func WriteAndCopy(data []byte, path string) error {
	// First write a "temp" file.
	tmpfile := filepath.Join(filepath.Dir(path), "."+filepath.Base(path))
	if err := ioutil.WriteFile(tmpfile, data, 0644); err != nil {
		return err
	}
	// Finally, copy that file to the correct location.
	return os.Rename(tmpfile, path)
}
