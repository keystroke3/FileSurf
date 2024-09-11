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

var DefaultAddr = ":10002"

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

type BoolStr string

func (s *BoolStr) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *BoolStr) Set(value string) error {
	if value == "" {
		*s = BoolStr(DefaultAddr)
	}
	return nil
}

func remotizeHomePaths(paths []string) ([]string, error) {
	clean_paths := []string{}
	for _, path := range paths {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("unable to determine local home directory")
		}
		if strings.HasPrefix(path, home) {
			clean_paths = append(clean_paths, strings.Replace(path, home, "~", 1))
		} else {
			clean_paths = append(clean_paths, path)
		}
	}
	return clean_paths, nil
}

func handleCommand(args CmdArgs) (string, error) {
	for _, path := range args.Paths {
		_, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("path not found '%v'\n", path)
			}
			return "", err
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
	return fmt.Sprint(strings.Join(allPaths, "\n")), nil
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
		if strings.HasPrefix(results.Error, "path not found") {
			fmt.Printf(strings.ReplaceAll(results.Error, "path not found", "path not found on "+addr))
		} else {
			fmt.Println("Remote daemon returned an error:", results.Error)
		}
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

	var vgrep string
	flag.StringVar(&vgrep, "v", "", "excludes paths match that match regex pattern")
	flag.StringVar(&vgrep, "vgrep", "", "excludes paths match that match regex pattern")

	var host string
	flag.StringVar(&host, "host", "", "address for a remote filesurf instance to use instead of local")

	var serve string
	flag.StringVar(&serve, "s", "", "telnet address to listen for commands")
	flag.StringVar(&serve, "serve", "", "telnet address to listen for commands")

	flag.Parse()
	argPaths := flag.Args()
	paths = append(paths, argPaths...)

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

	if serve != "" {
		TCPListen(string(serve))
		return
	}

	if host != "" {
		net_paths, err := remotizeHomePaths(cmd.Paths)
		cmd.Paths = net_paths
		if err != nil {
			fmt.Println(err)
			return
		}
		results := cmdOverTCP(cmd, host)
		fmt.Println(results)
		return
	}
	results, err := handleCommand(cmd)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(results)
}
