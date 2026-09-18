package web

import (
	"io/fs"
	"net/http"
)

type noDirectoryListingFileSystem struct {
	fileSystem http.FileSystem
}

func (f noDirectoryListingFileSystem) Open(
	name string,
) (http.File, error) {
	file, err := f.fileSystem.Open(name)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()

		return nil, err
	}

	if info.IsDir() {
		_ = file.Close()

		return nil, fs.ErrNotExist
	}

	return file, nil
}

// NewStaticFileHandler serves files while hiding directory contents.
func NewStaticFileHandler(root string) http.Handler {
	return http.FileServer(
		noDirectoryListingFileSystem{
			fileSystem: http.Dir(root),
		},
	)
}
