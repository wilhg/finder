package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/wsxiaoys/terminal/color"
)

type Result struct {
	Line    int
	Content string
}

type ResultList []Result

func (list *ResultList) Add(r Result) {
	list = append(list, r)
}
func (list *ResultList) FindByLine(line int) Result {
	for _, r := range list {
		if r.Line == line {
			return r
		}
	}
	return nil
}

func (list *ResultList) fristItem() Result {
	return list[0]
}
func (list *ResultList) lastItem() Result {
	return list[len(list)-1]
}

func (list *ResultList) Render(n int) ResultList {
	group := ResultList{}
	groups := []ResultList{} // [1,2,3,5,7,11,13,17] => [[1,2,3,5,7], [11, 13], [19]]
	for _, item := range list {
		if len(group) == 0 {
			group = append(group, item)
			continue
		}
		if diff := item.Line - group.lastItem().Line; diff <= 2*n {
			group.Add(item)
		} else {
			groups = append(groups, group)
			group = ResultList{}
			group = append(group, item)
		}
	}

	outputLines := ResultList{}
	for _, g := range groups {
		first := g.fristItem()
		last := g.lastItem()
		if head := first - n; head < 0 {
			head = 0
		}
		tail := last + n
		for i := head; i <= tail; i++ {
			if a := list.FindByLine(i); a == nil {
				a = Result{Line: i}
			}
			outputLines.Add(a)
		}
	}
	//TODO: render it
	return outputLines // [9-15, 17-21]
}

func matchText(expr, filename string) ResultList {
	const N = 2
	var (
		H = []byte("@B") // highlight
		R = []byte("@|") // reset
		J = []byte("")   // bytes joiner
	)
	re, _ := regexp.Compile(expr)
	f, _ := os.Open(filename)
	scanner := bufio.NewScanner(f)
	defer f.Close()

	resultList := ResultList{}
	for line := 1; scanner.Scan(); line++ {
		content := scanner.Bytes()
		indexes := re.FindAllSubmatchIndex(content, -1)
		if indexes == nil {
			continue
		}
		offset := 0 // the color code would crease length
		for _, index := range indexes {
			head := index[0] + offset
			tail := index[1] + offset
			offset += len(H) + len(R)
			total := [][]byte{content[0:head], H, content[head:tail], R, content[tail:]}
			content = bytes.Join(total, J)
		}
		resultList.Add(Result{line, content})
	}
	results := resultList.Render(2)

	for cousor, line := 0, 1; scanner.Scan(); line++ {
		if line != results[cousor].Line {
			continue
		} else if results[cousor].Content != "" {
			color.Printf("@c%d@|:\t", line)
			color.Println(results[cousor].Content)
		} else {
			color.Printf("@c%d@|\t", line)
			color.Println(scanner.Text())
			if results[cousor].Line-results[cousor-1].Line > 1 {
				fmt.Println(":")
			}
		}
		cousor++
	}

	return resultList
}

func walk(expr, path string) {
	if err := filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		// pick(expr, path)
		return nil
	}); err != nil {
		fmt.Printf("filepath.Walk() returned %v\n", err)
	}
}

func routineKeeper(done chan bool) {
	for {
		runtime.Gosched()
		select {
		case <-done:
			break
		case <-time.After(100 * time.Millisecond):
			fmt.Println("---- Search Over ----")
			return
		}
	}
}

func fullTextSearch(expr, path string, max int) {
	count := 0
	filename := make(chan string, max) // here should use chan
	done := make(chan bool, max)
	defer close(filename)
	defer close(done)

	go func() {
		for {
			select {
			case fn := <-filename:
				matchText(expr, fn).Render(2)
				done <- true
			}
		}
	}()

	filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if count >= max || f.IsDir() {
			return nil
		}
		filename <- path
		count++
		return nil
	})
	routineKeeper(done)
}

func main() {
	runtime.GOMAXPROCS(8)
	flag.Parse()
	expr := flag.Arg(0)
	currentPath, _ := os.Getwd()

	fullTextSearch(expr, currentPath, 1000)
}
