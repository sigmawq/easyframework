package easyframework

import (
	"fmt"
	"os"
	"path/filepath"
)

func GetLogList() []string {
	var logs []string
	filepath.Walk("logs", func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			logs = append(logs, info.Name())
		}
		return nil
	})

	return logs
}

func GetLog(name string, filter string, height int) string {
	logtext, _ := os.ReadFile(fmt.Sprintf("logs/%v", name)) // todo: buffered read
	return string(logtext)
}
