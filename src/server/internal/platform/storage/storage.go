package storage

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

var ErrUnsupportedType = errors.New("only JPEG, PNG and GIF images are allowed")


//! here, a better implemention wouldve been turning the image given to *.webp instead 
//? because it offers better compression and image quality compared to traditional formats
var allowed = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
}

type Store struct {
	dir   string
	files fs.FS // read-only view of dirconfig
}

func New(dir string) (*Store, error) {
	// execute and read for other and group; owner everything
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}
	return &Store{dir: dir, files: os.DirFS(dir)}, nil
}

// save checks the real file type, writes it under a random name and returns that name
// the name is what goes in the db (avatar_path, image_path); the file is served at /uploads/<name>
func (s *Store) Save(src io.Reader) (string, error) {
	// never trust the file extension or the client's Content-Type header: look at the bytes.
	// DetectContentType only needs the first 512
	head := make([]byte, 512)
	n, err := io.ReadFull(src, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read upload: %w", err)
	}
	head = head[:n]

	ext, ok := allowed[http.DetectContentType(head)]
	if !ok {
		return "", ErrUnsupportedType
	}

	// a random name means no path traversal (../../etc) and no two uploads overwriting each other
	name := rand.Text() + ext
	path := filepath.Join(s.dir, name)

	// O_EXCL: fail instead of overwriting if the name somehow exists already
	dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create upload: %w", err)
	}

	// we already consumed the first bytes, so glue them back in front of the rest of the stream
	if _, err := io.Copy(dst, io.MultiReader(bytes.NewReader(head), src)); err != nil {
		dst.Close()
		os.Remove(path)
		return "", fmt.Errorf("write upload: %w", err)
	}
	if err := dst.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("close upload: %w", err)
	}
	return name, nil
}

// Delete removes a saved file; used to clean up when the db insert after a Save fails
func (s *Store) Delete(name string) error {
	return os.Remove(filepath.Join(s.dir, name))
}

// Serve is the handler for GET /uploads/{name}.
// the mux's {name} wildcard can't contain "/", and fs.FS rejects "..", so this can't escape the folder
func (s *Store) Serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFileFS(w, r, s.files, r.PathValue("name"))
}
