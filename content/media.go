package content

import (
	"errors"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
)

var ErrInvalidMediaPath = errors.New("invalid media path")
var ErrMediaOutsideRoot = errors.New("media path outside root")

// ResolveMediaPath maps a URL-relative media path to an existing regular file
// below root. It evaluates symlinks before checking the boundary so a link
// cannot turn the media endpoint into an arbitrary file reader.
func ResolveMediaPath(root, relativePath string) (string, error) {
	clean, err := cleanRelativePath(relativePath)
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(absRoot, filepath.FromSlash(clean))
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(absRoot, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrMediaOutsideRoot
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", ErrInvalidMediaPath
	}
	return resolved, nil
}

func cleanRelativePath(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || strings.ContainsRune(value, 0) || strings.HasPrefix(value, "/") {
		return "", ErrInvalidMediaPath
	}
	clean := pathpkg.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
		return "", ErrInvalidMediaPath
	}
	return clean, nil
}
