package utils

import (
	"bufio"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
)

func ListFiles(root, pattern string) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		} else if d.IsDir() {
			return nil
		} else if matched, err := filepath.Match(pattern, filepath.Base(path)); err != nil {
			return err
		} else if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func CopyFile(src, dest string) error {
	buf, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(dest), 0755)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dest, buf, 0644)
}

func CopyFileToDir(src, destDir string) (string, error) {
	filename := filepath.Base(src)
	dest := filepath.Join(destDir, filename)
	return dest, CopyFile(src, dest)
}

func ReadLines(file string) ([]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	err = f.Close()
	if err != nil {
		return nil, err
	}
	return lines, nil
}
