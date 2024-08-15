package helpers

import (
	"fmt"
	"os"
	"time"
)

func TouchFile(filePath string) error {
	_, statErr := os.Stat(filePath)
	if os.IsNotExist(statErr) {
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("can't create file %q: %w", filePath, err)
		}
		err = file.Close()
		if err != nil {
			return fmt.Errorf("can't close file %q", filePath)
		}

		return nil
	}

	currentTime := time.Now().Local()
	err := os.Chtimes(filePath, currentTime, currentTime)
	if err != nil {
		return fmt.Errorf("can't change time for file %q to %q", filePath, currentTime)
	}

	return nil
}
