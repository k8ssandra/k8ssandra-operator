package utils

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
)

// ListFiles lists all files whose names match pattern in root directory and its subdirectories.
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

// CopyFile copies file src to file dest. File src must exist. File dest doesn't need to exist nor its parent directories,
// they will be created if required. Existing dest file will be overwritten.
func CopyFile(src, dest string) error {
	buf, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(dest), 0755)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, buf, 0644)
}

// CopyFileToDir copies file src to directory destDir. File src must exist. Directory destDir doesn't need exist nor its
// parent directories, they will be created if required. The destination file will have the same name as src. If that
// file already exists, it will be overwritten.
func CopyFileToDir(src, destDir string) (string, error) {
	filename := filepath.Base(src)
	dest := filepath.Join(destDir, filename)
	return dest, CopyFile(src, dest)
}

// ReadLines reads all lines in the given file.
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
