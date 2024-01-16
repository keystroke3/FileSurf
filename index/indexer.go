package index

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var mimeJson = "extToMime.json"

type File struct {
	Id        string
	Name      string
	Directory string
	MimeType  string
	Size      int64
	Modified  time.Time
}

func loadMimes() map[string]string {
	mimes := make(map[string]string)
	f, err := os.Open(mimeJson)
	if err != nil {
		return mimes
	}
	defer f.Close()
	mimeBytes, _ := io.ReadAll(f)
	json.Unmarshal(mimeBytes, &mimes)
	return mimes
}

var Mimes = loadMimes()

type Indexer interface {
	Add(path string) error
	Remove(path string) error
	Move(f *File, from string, to string) error
	AllPaths() []string
	PathMatch(field string, val string) []string
}

func hash(s string) string {
	return string(md5.New().Sum([]byte(s)))
}

func mimeFromExt(s string, m map[string]string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return ""
	}
	return m[parts[len(parts)-1]]
}

type MemIndex struct {
	Files   map[string]*File
	cfg     map[string]string
	current string
}

// Adds a new `File` entry to the index
func (i *MemIndex) Add(path string, f fs.DirEntry, err error) error {
	cfgIgnore := strings.ReplaceAll(i.cfg["ignore"], ", ", ",")
	ignore := strings.Split(cfgIgnore, ",")
	stat, err := f.Info()
	if err != nil {
		return err
	}
	if stat.IsDir() {
		for _, i := range ignore {
			if i == path {
				return fs.SkipDir
			}
		}
	}
	fullPath := filepath.Join(i.current, path)
	id := hash(fullPath)
	file := File{
		Id:        id,
		Name:      fullPath,
		Directory: filepath.Dir(path),
		Size:      stat.Size(),
		MimeType:  mimeFromExt(stat.Name(), Mimes),
		Modified:  stat.ModTime(),
	}

	i.Files[id] = &file
	return nil
}

func (i *MemIndex) Remove(path string) error {
	delete(i.Files, hash(path))
	return nil
}

// Relocates a file form one directory to another
// In practice it just changes the directory value in the file
func (i *MemIndex) Move(from string, to string) error {
	f, set := i.Files[hash(from)]
	if !set {
		return fmt.Errorf("path %v not found in index", from)
	}
	_, err := os.Stat(to)
	if err != nil {
		return err
	}
	f.Id = hash(to)
	f.Directory = to
	i.Remove(from)
	i.Files[f.Id] = f
	return nil
}

// Returns all the files. Do not use this unless it is absolutely neeeded
func (i *MemIndex) AllPaths() []string {
	paths := []string{}
	for _, p := range i.Files {
		paths = append(paths, p.Name)
	}
	return paths
}

// Returns all the `Files` with fields that match the value given
// Available fields are all the string fields in `File`
func (i *MemIndex) PathMatch(path string) []string {
	paths := []string{}
	for _, p := range i.Files {
		if strings.HasPrefix(p.Name, path) {
			paths = append(paths, p.Name)
		}
	}
	return paths
}

func Walk(paths []string, fn func(path string, d fs.DirEntry, err error) error) {
	for _, p := range paths {
		d := os.DirFS(p)
		fs.WalkDir(d, ".", fn)
	}
}
