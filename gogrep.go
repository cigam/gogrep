/*
gogrep implements a simple parallel grep like application.

Inspired by ack and the silver searcher.
*/
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
)

var query_str = flag.String("query", "", "help message for flagname")
var target_dir = flag.String("dir", "", "help message for flagname")
var query regexp.Regexp

// searchInFile does the actual search for the regex inside the file
func searchInFile(filename string) chan string {
	res := make(chan string)
	go func() {
		defer close(res)
		file, err := os.Open(filename)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed opening file:", err)
			return
		}

		scanner := bufio.NewScanner(file)
		for i := 1; scanner.Scan(); i++ {
			if query.Match(scanner.Bytes()) {
				res <- strconv.Itoa(i) + ": " + scanner.Text()
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
	}()
	return res
}

// searchInDir walks a directory and generated files to search. For every
// discovered subdirectory, it launches a new go routine (recursively).
func searchInDir(dir string, files chan string, first bool) chan os.FileInfo {
	res := make(chan os.FileInfo)
	go func() {
		defer close(res)
		if first {
			defer close(files)
		}
		p, _ := ioutil.ReadDir(dir)
		for i := 0; i < len(p); i++ {
			if p[i].IsDir() {
				subres := searchInDir(path.Join(dir, p[i].Name()), files, false)
				for j := range subres {
					files <- path.Join(dir, j.Name())
				}
			} else {
				files <- path.Join(dir, p[i].Name())
			}
		}
	}()
	return res
}

func main() {
	flag.Parse()

	query = *regexp.MustCompile(*query_str)
	files := make(chan string)
	searchInDir(*target_dir, files, true)
	for i := range files {
		res := searchInFile(i)
		found_something := false
		for result := range res {
			if !found_something {
				fmt.Println(i + ":")
				found_something = true
			}
			fmt.Println(result)
		}
	}
}
