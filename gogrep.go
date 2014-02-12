/*
gogrep implements a simple parallel grep like application.

Inspired by ack and the silver searcher.
*/
package main

import (
	"bufio"
	"container/list"
	"fmt"
	"github.com/docopt/docopt.go"
	"github.com/mgutz/ansi"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strconv"
	"sync"
	"time"
)

type HitPrinter func(int, int, string)

type Hit struct {
	Line, Col int
	Text      string
}

var query regexp.Regexp
var printLock sync.Mutex

var ignore_types = map[string]bool{
	"application/octet-stream": true,
}

var ignore_files = map[string]bool{
	".svn":              true,
	".agignore":         true,
	".gitignore":        true,
	".git":              true,
	".git/info/exclude": true,
	".hgignore":         true,
}

func noColorPrinter(line int, col int, text string) {
	fmt.Printf("%d: %s\n", line, text)
}

var redColor = ansi.ColorCode("red+b")
var yellowColor = ansi.ColorCode("yellow")
var resetColor = ansi.ColorCode("reset")

func colorPrinter(line int, col int, text string) {
	msg := redColor + strconv.Itoa(line) + ": " + resetColor + yellowColor + text + resetColor
	fmt.Println(msg)
}

// searchInFile does the actual search for the regex inside the file
func searchInFile(filename string, printer HitPrinter) {
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		log.Fatal(os.Stderr, "Failed opening file:", err)
		return
	}

	file_header := make([]byte, 512)
	file.Read(file_header)

	if ignore_types[http.DetectContentType(file_header)] {
		return
	}
	hits := list.New()
	scanner := bufio.NewScanner(file)
	for i := 1; scanner.Scan(); i++ {
		if query.Match(scanner.Bytes()) {
			hits.PushBack(Hit{i, 0, scanner.Text()})
		}
	}
	if hits.Len() > 0 {
		// We serielize the printing of the results for grouping per file
		//
		printLock.Lock()
		fmt.Println(filename)
		for e := hits.Front(); e != nil; e = e.Next() {
			printer(e.Value.(Hit).Line, 0, e.Value.(Hit).Text)
		}
		fmt.Println("")
		printLock.Unlock()
	}
}

// searchInDir walks a directory and generated files to search.
//
func searchInDir(dir string, files chan string) {
	p, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(os.Stderr, "Failed opening file:", err)
		return
	}
	for i := 0; i < len(p); i++ {
		if ignore_files[p[i].Name()] {
			continue
		}

		if p[i].IsDir() {
			searchInDir(path.Join(dir, p[i].Name()), files)
		} else {
			files <- path.Join(dir, p[i].Name())
		}
	}
}

func initializeIgnoreList() {
}

func searchFiles(files chan string) {
	// Use a pool of goroutines to search the files, otherwise we might reach
	// the max open files permitted
	//
	var wg sync.WaitGroup
	const OPEN_FILES = 64
	for i := 0; i < OPEN_FILES; i++ {
		wg.Add(1)
		go func() {
			for i := range files {
				searchInFile(i, colorPrinter)
			}
			wg.Done()
		}()
	}
	// wait for the workers to finish
	wg.Wait()
}

func searchPaths(paths []string, files chan string) {
	// Close the files channel
	defer close(files)
	if len(paths) == 0 {
		pwd, _ := os.Getwd()
		searchInDir(pwd, files)
	} else {
		for _, path := range paths {
			fi, _ := os.Stat(path)
			if fi.IsDir() {
				searchInDir(path, files)
			} else {
				files <- path
			}
		}
	}
}

func main() {
	start := time.Now()
	usage := `Recursively search for PATTERN in PATH

    usage: gogrep [options] PATTERN [PATH...]

	Arguments:
	  PATTERN  go regular expression

	Options:
	  -h --help            show this help message and exit
	  --version            show version and exit
	  -i --ignore-case     Match case insensitively
	  --ignore=PATTERNS    exclude files or directories which match these comma
	                       separated patterns [default: .svn,CVS,.bzr,.hg,.git]
	  --profile			   Run the go profiler on this run`

	arguments, _ := docopt.Parse(usage, nil, true, "0.1", false)
	if arguments["--profile"].(bool) {
		f, err := os.Create("gogrep.prof")
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	// Since we are mostly IO bound, use also the hyper threads
	// Should benchmark this though...
	//
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)

	query = *regexp.MustCompile(arguments["PATTERN"].(string))

	initializeIgnoreList()

	files := make(chan string)
	go searchPaths(arguments["PATH"].([]string), files)
	searchFiles(files)

	fmt.Println("Search finished in: ", time.Since(start))
}
