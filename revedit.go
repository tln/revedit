package main

import (
	"encoding/json"
	"errors"
	"fmt"
	diff "github.com/sergi/go-diff/diffmatchpatch"
	tmpl "github.com/tln/revedit/html/template"
	texttmpl "github.com/tln/revedit/text/template"
	"io/ioutil"
	"os"
	"strings"
)

func main() {
	// body
	usage := "usage: test_tmpl.go file.tmpl|file.html"
	if len(os.Args) < 2 {
		fmt.Println(usage)
		return
	}

	arg := os.Args[1]
	if strings.HasSuffix(arg, ".tmpl") {
		generateWithTmpl(arg)
	} else if strings.HasSuffix(arg, ".html") {
		updateTmplFromHtml(arg)
	} else {
		fmt.Println(usage)
		return
	}
}

func updateTmplFromHtml(inFilename string) {
	inHtml, err := ioutil.ReadFile(inFilename)
	if err != nil {
		fmt.Println("Error opening ", inFilename, " err:", err)
		return
	}

	origFilename := strings.Replace(inFilename, ".html", ".orig.html", -1)
	origHtml, origErr := ioutil.ReadFile(origFilename)
	if origErr != nil {
		fmt.Println("Error opening ", origFilename, " err:", origErr)
		return
	}

	traceFilename := strings.Replace(inFilename, ".html", ".trace.json", -1)
	traceJson, traceErr := ioutil.ReadFile(traceFilename)
	if origErr != nil {
		fmt.Println("Error opening ", origFilename, " err:", traceErr)
		return
	}
	trace := &texttmpl.Trace{}
	jsonErr := json.Unmarshal(traceJson, trace)
	if jsonErr != nil {
		fmt.Println("Error parsing ", traceFilename, " err:", jsonErr)
		return
	}

	doComparison(inHtml, origHtml, trace)
}

func doComparison(in, orig []byte, trace *texttmpl.Trace) {
	differ := diff.New()
	diffs := differ.DiffMain(string(orig), string(in), false)
	traceDiffs := matchDiffsToTraces(trace, diffs)

	appliedDiffs := make(AppliedDiffs)
	for _, trdiff := range traceDiffs {
		appliedDiffs.applyDiffs(trdiff)
	}
	appliedDiffs.saveDiffs()
}

type TraceWithDiffs struct {
	texttmpl.Traces
	diffs    []diff.Diff
	filename string
}

type AppliedDiffs map[string]*AppliedDiffEntry

type AppliedDiffEntry struct {
	buf      []byte
	filename string
	ops      []diffOp
	pos      int // current position
}

type diffOp struct {
	pos, offset int
}

func (a *AppliedDiffs) applyDiffs(tr TraceWithDiffs) error {
	if len(tr.diffs) == 0 {
		return errors.New("No diffs for segment")
	}
	if tr.InputType != 0 {
		// Can't apply diff to this type. If we have actionable diffs,
		// it's an error. Otherwise, we don't have anything to do.
		for _, d := range tr.diffs {
			if d.Type != diff.DiffEqual {
				return errors.New("Diffs for uneditable segment")
			}
		}
		return nil
	}

	e := a.makeEntry(tr.filename)
	e.setPos(tr.InputPos)
	for _, d := range tr.diffs {
		e.applyDiff(int(d.Type), d.Text)
	}
	return nil
}

func (e *AppliedDiffEntry) setPos(pos int) int {
	e.pos = pos
	for _, op := range e.ops {
		if op.pos < e.pos {
			//!!! is this correct?
			e.pos -= op.offset
		}
	}
	return e.pos
}

func (e *AppliedDiffEntry) applyDiff(op int, text string) {
	b := []byte(text)
	if op == diff.DiffEqual {
		e.pos += len(b)
		return
	}
	ofs := len(b)
	if op == diff.DiffInsert {
		e.buf = append(e.buf[:e.pos], append(b, e.buf[e.pos:]...)...)
	} else {
		e.buf = append(e.buf[:e.pos], e.buf[e.pos+ofs:]...)
		ofs = -ofs
	}
	e.ops = append(e.ops, diffOp{e.pos, ofs})
}

func (a AppliedDiffs) saveDiffs() error {
	for _, e := range a {
		if len(e.ops) == 0 {
			fmt.Println("File has not changed: ", newfilename)
			continue
		}
		newfilename := e.filename + ".new"
		err := ioutil.WriteFile(newfilename, e.buf, os.ModePerm)
		if err != nil {
			fmt.Println("Error writing", newfilename)
			return err
		}
		fmt.Println("Wrote", newfilename)
	}
	return nil
}

func (a AppliedDiffs) makeEntry(filename string) *AppliedDiffEntry {
	e, found := a[filename]
	if !found {
		buf, err := ioutil.ReadFile(filename)
		if err != nil {
			panic(err)
		}
		e = &AppliedDiffEntry{
			filename: filename,
			ops:      make([]diffOp, 1),
			buf:      buf,
		}
		a[filename] = e
	}
	return e
}

func matchDiffsToTraces(trace *texttmpl.Trace, diffs []diff.Diff) []TraceWithDiffs {
	result := make([]TraceWithDiffs, 0)
	ix := 0      // index of the diff we're looking at
	startix := 0 // starting index of the set of diffs that correspond to a trace
	length := 0  // length so far that the diffs contribute to the original doc (ie, text2)
	for _, tr := range trace.Traces {
		n := len(result)
		result = append(result, TraceWithDiffs{
			Traces:   tr,
			filename: trace.Names[tr.Ix],
		})
		cur := &result[n]

		if ix >= len(diffs) {
			// Ran out of diffs!
			continue
		}

		for {
			d := diffs[ix]
			ix += 1
			if d.Type != diff.DiffInsert {
				length += len(d.Text)
			}

			if length > tr.OutputLength {
				// split node
				cur.diffs = make([]diff.Diff, ix-startix)
				copy(cur.diffs, diffs[startix:ix])
				d := &cur.diffs[len(cur.diffs)-1]
				n := len(d.Text) - (length - tr.OutputLength)
				d.Text = d.Text[:n]
				ix -= 1
				diffs[ix].Text = diffs[ix].Text[n:]
				startix = ix
				length = 0
				break
			} else if length == tr.OutputLength || ix >= len(diffs) {
				cur.diffs = diffs[startix:ix]
				startix = ix
				length = 0
				break
			}

		}
	}
	if ix < len(diffs) {
		fmt.Println("Unused diffs:", diffs[ix:])
	}
	return result
}

func generateWithTmpl(inTmpl string) {
	t, err := tmpl.New(inTmpl).ParseFiles(inTmpl)
	if err != nil {
		fmt.Println("err in ParseFiles: ", err, " args:", os.Args)
		return
	}

	// Write file.html
	htmlFile, f1 := openFile(inTmpl, ".html")
	defer f1.Close()
	tr, err := t.TraceExecute(f1, "")
	if err != nil {
		fmt.Println("err in TraceExecte: ", err, " args:", os.Args)
		return
	}

	// Write file.orig.html
	// !!! should just copy file
	origFile, f2 := openFile(inTmpl, ".orig.html")
	defer f2.Close()
	tr, err = t.TraceExecute(f2, "")
	if err != nil {
		fmt.Println("err in TraceExecte: ", err, " args:", os.Args)
		return
	}

	// Write trace.json
	traceFile, f3 := openFile(inTmpl, ".trace.json")
	f3.Write(tr.JSON())
	f3.Close()

	fmt.Println("Wrote ", htmlFile, " and ", origFile, " and ", traceFile)
	fmt.Println("Now run: $EDITOR", htmlFile)
	fmt.Println("Then run: revedit", htmlFile)
}

func openFile(tmpl, newExt string) (string, *os.File) {
	out := strings.Replace(tmpl, ".tmpl", newExt, -1)
	if out == tmpl {
		panic("Input file must be named *.tmpl")
	}

	outFile, err := os.Create(out)
	if err != nil {
		fmt.Println("Can't create ", out, " error: ", err)
		panic("Can't continue")
	}

	return out, outFile
}
