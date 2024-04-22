package main

import (
	"encoding/json"
	"filesurf/index"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

// StringSliceVar is a custom type that implements the flag.Value interface
// It allows us to collect multiple values for the same flag into a slice

type CmdArgs struct {
	Depth       int
	DirMode     bool
	Grep        string
	IgnorePaths []string
	Paths       []string
	ShowHidden  bool
	Vgrep       string
}

type StringSliceVar []string

func (s *StringSliceVar) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *StringSliceVar) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func remotizeHomePaths(args *CmdArgs) error {
	clean_paths := []string{}
	for _, path := range args.Paths {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("unable to determine local home directory")
		}
		if strings.HasPrefix(path, home) {
			clean_paths = append(clean_paths, strings.Replace(path, home, "~", 1))
		} else {
			clean_paths = append(clean_paths, path)
		}
	}
	args.Paths = clean_paths
	return nil
}


func handleCommand(args CmdArgs) string {
	for _, path := range args.Paths {
		_, err := os.Stat(path)
		if err != nil {
			return fmt.Sprintf("path not found '%v'\n", path)
		}
	}

	memIndex := index.NewMemIndex(args.Paths, args.IgnorePaths, args.ShowHidden, args.Depth)
	index.Walk(args.Paths, &memIndex.Root, &memIndex.Current, memIndex.Add)

	var allPaths []string
	if args.DirMode {
		allPaths = memIndex.AllDirs()
	} else {
		allPaths = memIndex.AllFiles()
	}
	if args.Grep != "" {
		allPaths = index.Some(allPaths, args.Grep)
	}
	if args.Vgrep != "" {
		allPaths = index.Some(allPaths, args.Vgrep, false)
	}
	return fmt.Sprint(strings.Join(allPaths, "\n"))
}

func cmdOverTCP(args CmdArgs, addr string) string {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Println("unable to connect to address", addr, "target may be offline")
		os.Exit(1)
	}
	defer conn.Close()
	msg, err := json.Marshal(args)
	if err != nil {
		fmt.Println("unable to marshal command, ", err)
		os.Exit(1)
	}
	conn.Write(msg)
	resp, err := io.ReadAll(conn)
	var results NetResponse
	err = json.Unmarshal(resp, &results)
	if err != nil {
		fmt.Println("Unable to read daemon resopnse, ", err)
		os.Exit(1)
	}
	if results.Error != "" {
		fmt.Println("Remote daemon returned an error:", results.Error)
		os.Exit(1)
	}
	return results.Paths

}

func main() {
	var paths StringSliceVar
	flag.Var(&paths, "p", "path(s) to search. Use multiple times for more paths")
	flag.Var(&paths, "paths", "path(s) to search. Use multiple times for more paths")

	var ignorePaths StringSliceVar
	flag.Var(&ignorePaths, "i", "path(s) to ignore in indexing. Use multiple times for more paths")
	flag.Var(&ignorePaths, "ignore", "path(s) to ignore in indexing. Use multiple times for more paths")

	var showHidden bool
	flag.BoolVar(&showHidden, "H", false, "include hidden directories in scan")
	flag.BoolVar(&showHidden, "hidden", false, "include hidden directories in scan")

	var dirMode bool
	flag.BoolVar(&dirMode, "d", false, "Return only directories")
	flag.BoolVar(&dirMode, "dirs", false, "Return only directories")

	var depth int
	flag.IntVar(&depth, "D", -1, "How many nested directories to index")
	flag.IntVar(&depth, "depth", -1, "How many nested directories to index")

	var grep string
	flag.StringVar(&grep, "g", "", "show path matches that match regex pattern")
	flag.StringVar(&grep, "grep", "", "show path files matches that match regex pattern")

	var host string
	flag.StringVar(&host, "host", "", "HTTP address to use. Will listen on address in daemon mode and connect in command mode")

	var vgrep string
	flag.StringVar(&vgrep, "v", "", "excludes paths match that match regex pattern")
	flag.StringVar(&vgrep, "vgrep", "", "excludes paths match that match regex pattern")

	var daemon bool
	flag.BoolVar(&daemon, "deamon", false, "Launch filesurf daemon")
	flag.BoolVar(&daemon, "demon", false, "Launch filesurf demon")

	flag.Parse()

	if len(paths) == 0 {
		path, err := os.Getwd()
		if err != nil {
			fmt.Println("could not load paths, ", err)
		}
		paths = append(paths, path)
	}

	cmd := CmdArgs{
		Depth:       depth,
		DirMode:     dirMode,
		Grep:        grep,
		IgnorePaths: ignorePaths,
		Paths:       paths,
		ShowHidden:  showHidden,
		Vgrep:       vgrep,
	}

	if host == "" {
		results := handleCommand(cmd)
		fmt.Println(results)
		return
	}
	if !daemon {
        err := remotizeHomePaths(&cmd)
        if err != nil{
            fmt.Println(err)
            return
        }
		results := cmdOverTCP(cmd, host)
		fmt.Println(results)
		return
	} else {
		TCPListen(host)
	}
}
