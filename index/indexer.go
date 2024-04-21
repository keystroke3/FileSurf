package index

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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

func NewMemIndex(paths []string, ignore []string, showHidden bool, depth int) *MemIndex {
	return &MemIndex{
		Files:      make(map[string]*File),
		Dirs:       make(map[string]bool),
		paths:      paths,
		ignore:     ignore,
		showHidden: showHidden,
		depth:      depth,
	}
}

type MemIndex struct {
	Files      map[string]*File
	Dirs       map[string]bool
	Current    string
	Root       string
	paths      []string
	ignore     []string
	showHidden bool
	depth      int
}

// Adds a new `File` entry to the index
func (i *MemIndex) Add(path string, f fs.DirEntry, err error) error {
	if path == "." {
		return nil
	}
	stat, err := f.Info()
	if err != nil {
		return err
	}
	fullPath := filepath.Join(i.Current, path)
	relPath, err := filepath.Rel(i.Root, fullPath)
	if err != nil {
		fmt.Println("could not determine relative path,", err)
		os.Exit(1)
	}
	depth := strings.Count(relPath, string(os.PathSeparator))
	if stat.IsDir() {
		if depth == i.depth {
			return fs.SkipDir
		}
		for _, i := range i.ignore {
			if i == path {
				return fs.SkipDir
			}
		}
		if !i.showHidden && strings.HasPrefix(path, ".") {
			return fs.SkipDir
		}
		i.Dirs[fullPath] = true
		return nil
	}
	if !i.showHidden && strings.HasPrefix(stat.Name(), ".") {
		return nil
	}
	id := hash(fullPath)
	file := File{
		Id:        id,
		Name:      fullPath,
		Directory: filepath.Dir(path),
		Size:      stat.Size(),
		// MimeType:  mimeFromExt(stat.Name(), Mimes),
		Modified: stat.ModTime(),
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
func (i *MemIndex) AllFiles() []string {
	paths := []string{}
	for _, p := range i.Files {
		paths = append(paths, p.Name)
	}
	return paths
}

func (i *MemIndex) AllDirs() []string {
	dirs := []string{}
	for p := range i.Dirs {
		dirs = append(dirs, p)
	}
	return dirs
}

// Returns only the []string values that contain substring v
//
// if optional `mode > 0`, reverses the match
func Some(s []string, v string, inclusive ...bool) []string {
	include := true
	re, err := regexp.Compile(v)
	if err != nil {
		fmt.Printf("unable to read regex: '%v', %v\n", v, err)
		os.Exit(1)
	}

	if len(inclusive) > 0 && !inclusive[0] {
		include = false
	}
	res := []string{}
	if len(s) == 0 {
		return res
	}
	if v == "" {
		return s
	}
	for _, p := range s {
		match := re.FindString(p)
		if match != "" {
			if include {
				res = append(res, p)
			}
		} else if !include {
			res = append(res, p)
		}
	}
	return res
}

func Walk(paths []string, root *string, current *string, fn func(path string, d fs.DirEntry, err error) error) {
	for _, p := range paths {
		d := os.DirFS(p)
		*current = p
		*root = p
		fs.WalkDir(d, ".", fn)
	}
}
