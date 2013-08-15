package main

import (
	"fmt"
	"io/ioutil"
)

func searchInDir(path string, done chan int) chan string {
	res := make(chan string)
	go func() {
		p, _ := ioutil.ReadDir(path)
		for i := 0; i < len(p); i++ {
			res <- p[i].Name()
		}
		done <- 0
	}()
	return res
}

func main() {
	done := make(chan int)
	res := searchInDir("/tmp", done)
	for {
		select {
		case v := <-res:
			fmt.Println(v)
		case <-done:
			fmt.Println("Finished searching")
			return
		}

	}
}
