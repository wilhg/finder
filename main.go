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

var (
	SEARCH_MAX    int
	IS_FULLTEXT   bool
	CONTEXT_RANGE int

	H = []byte("@{!C}") // highlight
	R = []byte("@|")    // reset
)

func Init() {
	flag.IntVar(&SEARCH_MAX, "max", 1000, "The maximum number of search")
	flag.IntVar(&CONTEXT_RANGE, "n", 2, "The lines of context to show")
	flag.BoolVar(&IS_FULLTEXT, "f", false, "Use fulltext search")
	runtime.GOMAXPROCS(runtime.NumCPU())
}

type Result struct {
	Line    int
	Content string
}
type ResultList []Result

func (list *ResultList) Add(r Result) {
	*list = append(*list, r)
}
func (list ResultList) FindByLine(line int) (error, Result) {
	for _, r := range list {
		if r.Line == line {
			return nil, r
		}
	}
	return fmt.Errorf("Not Found"), Result{}
}

func (list ResultList) getFristItem() Result {
	return list[0]
}
func (list ResultList) getLastItem() Result {
	return list[len(list)-1]
}

func (list ResultList) Render(n int) ResultList {
	group := ResultList{}
	groups := []ResultList{} // [1,2,3,5,7,11,13,17] => [[1,2,3,5,7], [11, 13], [19]]
	for _, item := range list {
		if len(group) > 0 && item.Line-group.getLastItem().Line > 2*n {
			groups = append(groups, group)
			group = ResultList{}
		}
		group.Add(item)
	}
	if len(group) > 0 {
		groups = append(groups, group)
	}

	outputLines := ResultList{}
	for _, g := range groups {
		head := g.getFristItem().Line - n
		tail := g.getLastItem().Line + n
		if head < 0 {
			head = 0
		}
		for i := head; i <= tail; i++ {
			err, a := list.FindByLine(i)
			if err != nil {
				a = Result{Line: i}
			}
			outputLines.Add(a)
		}
	}
	return outputLines // [9-15, 17-21]
}

func fulltextSearch(re regexp.Regexp, filename string) {
	f, _ := os.Open(filename)
	defer f.Close()
	scanner := bufio.NewScanner(f)

	resultList := ResultList{}
	for line := 1; scanner.Scan(); line++ {
		content := scanner.Bytes()
		// to avoid inject problem, but the cost is change input from @ to @@
		content = bytes.Replace(content, []byte("@"), []byte("@@"), -1)
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
			content = bytes.Join(total, []byte(""))
		}
		resultList.Add(Result{line, string(content)})
	}

	results := resultList.Render(CONTEXT_RANGE)
	if len(results) == 0 {
		return
	}
	color.Printf("@{!y}" + filename + "@|\n")
	f.Seek(0, 0)
	scanner2 := bufio.NewScanner(f)
	for cousor, line := 0, 1; cousor < len(results) && scanner2.Scan(); line++ {
		if line != results[cousor].Line {
			continue
		}
		fmt.Print("  ")
		if results[cousor].Content != "" {
			color.Printf("@{!c}%d@|:\t", line)
			color.Println(results[cousor].Content)
		} else {
			color.Printf("@c%d@|\t", line)
			fmt.Println(scanner2.Text())
		}
		if cousor+1 < len(results) && results[cousor+1].Line-results[cousor].Line != 1 {
			fmt.Print("  ")
			for num := results[cousor+1].Line; num > 0; num /= 10 {
				fmt.Print(".")
			}
			fmt.Println()
		}
		cousor++
	}
	fmt.Println()
}
func filenameSearch(re regexp.Regexp, filename string) {
	content := []byte(filename)
	indexes := re.FindAllSubmatchIndex(content, -1)
	if indexes == nil {
		return
	}
	offset := 0 // the color code would crease length
	for _, index := range indexes {
		head := index[0] + offset
		tail := index[1] + offset
		offset += len(H) + len(R)
		total := [][]byte{content[0:head], H, content[head:tail], R, content[tail:]}
		content = bytes.Join(total, []byte(""))
	}
	color.Println(string(content))
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
func search(expr, path string, max int) {
	filename := make(chan string, max)
	done := make(chan bool, max)
	defer close(filename)
	defer close(done)

	var searchFunc func(regexp.Regexp, string)
	if IS_FULLTEXT {
		searchFunc = fulltextSearch
	} else {
		searchFunc = filenameSearch
	}

	go func() {
		re, err := regexp.Compile(expr)
		if err != nil {
			fmt.Println(err)
			return
		}
		for {
			fn := <-filename
			searchFunc(*re, fn)
			done <- true
		}
	}()
	defer routineKeeper(done)
	count := 0
	filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if count < max && !f.IsDir() {
			filename <- path
			count++
		}
		return nil
	})
}

func main() {
	Init()
	flag.Parse()
	currentPath, _ := os.Getwd()
	// to avoid inject problem, but the cost is change input from @ to @@
	expr := string(bytes.Replace([]byte(flag.Arg(0)), []byte("@"), []byte("@@"), -1))
	search(expr, currentPath, SEARCH_MAX) // fullTextSearch(expr, currentPath, 1000)
}
