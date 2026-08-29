package stage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func StageRepository(srcDir, destDir string, includeGit bool, excludePatterns []string) error {
	info, err := os.Stat(srcDir)
	if err != nil {
		return fmt.Errorf("repo path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("repo path is not a directory: %s", srcDir)
	}
	if err := os.RemoveAll(destDir); err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	excludes := append([]string{".filter"}, excludePatterns...)
	if !includeGit {
		excludes = append(excludes, ".git")
	}
	return copyTree(srcDir, destDir, excludes)
}

func copyTree(src, dst string, excludePatterns []string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if shouldExclude(rel, excludePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func shouldExclude(rel string, patterns []string) bool {
	parts := strings.Split(rel, string(os.PathSeparator))
	for _, part := range parts {
		for _, pat := range patterns {
			if part == pat {
				return true
			}
		}
	}
	return false
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
