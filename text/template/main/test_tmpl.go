package main

import (
	"fmt"
	tmpl "github.com/tln/revedit/text/template"
	"os"
)

func main() {
	// body
	fmt.Println("Hello world")
	if len(os.Args) < 2 {
		fmt.Println("usage: test_tmpl.go file1 ...")
	}
	t, err := tmpl.New(os.Args[1]).ParseFiles(os.Args[1:]...)
	if err != nil {
		fmt.Println("err in ParseFiles: ", err, " args:", os.Args)
		return
	}
	tr, err := t.TraceExecute(os.Stdout, "")
	if err != nil {
		fmt.Println("err in TraceExecte: ", err, " args:", os.Args)
		return
	}
	fmt.Println("trace:", tr)
}
