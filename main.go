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
	flag.Var(&paths, "paths", "Path(s) to search. Use multiple times for more paths")

	var ignorePaths StringSliceVar
	flag.Var(&ignorePaths, "i", "Path(s) to ignore in indexing. Use multiple times for more paths")
	flag.Var(&ignorePaths, "ignore", "Path(s) to ignore in indexing. Use multiple times for more paths")

	var showHidden bool
	flag.BoolVar(&showHidden, "H", false, "include hidden directories in scan")
	flag.BoolVar(&showHidden, "hidden", false, "include hidden directories in scan")

	var dirMode bool
	flag.BoolVar(&dirMode, "d", false, "Return only directories")
	flag.BoolVar(&dirMode, "only-dirs", false, "Return only directories")

	var depth int
	flag.IntVar(&depth, "D", -1, "How many nested directories to index")
	flag.IntVar(&depth, "depth", -1, "How many nested directories to index")

	var grep string
	flag.StringVar(&grep, "g", "", "show path matches that match regex pattern")
	flag.StringVar(&grep, "grep", "", "show path files matches that match regex pattern")

	var vgrep string
	flag.StringVar(&vgrep, "v", "", "excludes paths match that match regex pattern")
	flag.StringVar(&vgrep, "vgrep", "", "excludes paths match that match regex pattern")

	flag.Parse()

	if len(paths) == 0 {
		path, err := os.Getwd()
		if err != nil {
			fmt.Println("could not load paths, ", err)
		}
		paths = append(paths, path)
	}
	for _, path := range paths {
		_, err := os.Stat(path)
		if err != nil {
			fmt.Printf("path not found '%v'\n", path)
			os.Exit(1)
		}
	}
	memIndex := index.NewMemIndex(paths, ignorePaths, showHidden, depth)

	index.Walk(paths, &memIndex.Root, &memIndex.Current, memIndex.Add)
	var allPaths []string
	if dirMode {
		allPaths = memIndex.AllDirs()
	} else {
		allPaths = memIndex.AllFiles()
	}
	if grep != "" {
		allPaths = index.Some(allPaths, grep)
	}
	if vgrep != "" {
		allPaths = index.Some(allPaths, vgrep, false)
	}
	fmt.Println(strings.Join(allPaths, "\n"))

}
