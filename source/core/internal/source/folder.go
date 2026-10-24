package source

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"docssearch/core/internal/chunk"
)

var ErrUnsafePath = errors.New("unsafe path")

type FolderFile struct {
	Name string
	Body []byte
}

type FolderListing struct {
	Files   []FolderFile
	Skipped []string
}

// Read_Folder читает один уровень каталога. Сегмент ".." в пути отклоняется.
func Read_Folder(rawPath string) (FolderListing, error) {
	if strings.TrimSpace(rawPath) == "" || strings.Contains(rawPath, "..") {
		return FolderListing{}, ErrUnsafePath
	}
	info, err := os.Stat(rawPath)
	if err != nil || !info.IsDir() {
		return FolderListing{}, ErrUnsafePath
	}
	entries, err := os.ReadDir(rawPath)
	if err != nil {
		return FolderListing{}, ErrUnsafePath
	}
	listing := FolderListing{Files: []FolderFile{}, Skipped: []string{}}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !chunk.Supports_DocumentText(name) {
			listing.Skipped = append(listing.Skipped, name)
			continue
		}
		body, readError := os.ReadFile(filepath.Join(rawPath, name))
		if readError != nil {
			listing.Skipped = append(listing.Skipped, name)
			continue
		}
		listing.Files = append(listing.Files, FolderFile{Name: name, Body: body})
	}
	return listing, nil
}
