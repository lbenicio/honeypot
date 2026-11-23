package main

import (
	"fmt"
	"path/filepath"
	"time"
)

func logFilePath(directory string, now time.Time) string {
	filename := fmt.Sprintf("fakessh-%s.log", now.Format("2006-01-02-15-04-05-000"))
	return filepath.Join(directory, filename)
}
