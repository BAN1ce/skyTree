package fsutil

import (
	"os"
)

func CreateDir(dir string) error {

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Directory does not exist, create it
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil

}
