package utils

import (
	"os"
	"path/filepath"
	"strings"
)

func FindGoModDir(args []string) (string, error) {
	startPath, err := findLongestStr(args)
	if err != nil {
		return "", err
	}

	dir := startPath
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		// 获取 dir 的父目录
		parentDir := filepath.Dir(dir)
		// 如果已经到达根目录，仍未找到 go.mod 文件，则返回错误
		if parentDir == dir {
			return "", os.ErrNotExist
		}
		dir = parentDir
	}
}

func findLongestStr(paths []string) (string, error) {
	var longestPath string
	longestLength := 0

	for _, path := range paths {
		if path == "builtin" {
			continue
		}
		// 分离 file=github.com/xxx 这样的 args
		parts := strings.Split(path, "=")
		path = parts[len(parts)-1]
		// 将 ./... 改为 ./ 的正常文件夹
		path = strings.ReplaceAll(path, "...", "")
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		if len(absPath) > longestLength {
			longestPath = absPath
			longestLength = len(absPath)
		}
	}

	return longestPath, nil
}
