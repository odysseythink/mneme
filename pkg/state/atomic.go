package state

import (
	"fmt"
	"math/rand"
	"os"
)

// AtomicWrite writes data to path via a temp file + fsync + rename.
func AtomicWrite(path string, data []byte) error {
	tmp := fmt.Sprintf("%s.tmp.%x", path, rand.Int63())
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if f, err := os.Open(tmp); err == nil {
		f.Sync()
		f.Close()
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
