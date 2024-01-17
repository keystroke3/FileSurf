package main

import (
	"filesurf/index"
	"flag"
	"fmt"
	"os"
	"strings"
)

// StringSliceVar is a custom type that implements the flag.Value interface
// It allows us to collect multiple values for the same flag into a slice
type StringSliceVar []string

func (s *StringSliceVar) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *StringSliceVar) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	var paths StringSliceVar
	flag.Var(&paths, "p", "Path(s) to search. Use multiple times for more paths")

	var ignorePaths StringSliceVar
	flag.Var(&ignorePaths, "i", "Path(s) to ignore in indexing. Use multiple times for more paths")

	var dirMode bool
	flag.BoolVar(&dirMode, "d", false, "Return only directories")

	var ignoreHidden bool
	flag.BoolVar(&ignoreHidden, "ignore-hidden", false, "Ignores hidden directories prefixed with '.'")

	flag.Parse()
	if len(paths) == 0 {
		path, err := os.Getwd()
		if err != nil {
			fmt.Println("could not load paths, ", err)
		}
		paths = append(paths, path)
	}

	memIndex := index.NewMemIndex(paths, ignorePaths, ignoreHidden)
	for _, path := range paths {
		_, err := os.Stat(path)
		if err != nil {
			fmt.Println("path not found", path)
			os.Exit(1)
		}
	}
	index.Walk(paths, &memIndex.Current, memIndex.Add)
	var allPaths []string
	if dirMode {
		allPaths = memIndex.AllDirs()

	} else {
		allPaths = memIndex.AllFiles()
	}
	fmt.Println(strings.Join(allPaths, "\n"))

}
