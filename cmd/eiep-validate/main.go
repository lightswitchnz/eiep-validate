package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"eiep-validate/internal/eiep"
)

// perKind is how many findings of one kind are printed before the rest are summarised.
const perKind = 3

type fileFinding struct {
	File string `json:"file"`
	eiep.Finding
}

type kind struct{ field, code string }

func main() {
	asJSON := flag.Bool("json", false, "print every finding as JSON")
	all := flag.Bool("all", false, "print every finding instead of summarising repeats")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: eiep-validate [--json] [--all] FILE...")
		fmt.Fprintln(os.Stderr, "Validates EIEP files strictly against the Electricity Authority spec.")
		fmt.Fprintln(os.Stderr, "Exit status: 0 valid, 1 findings, 2 usage or read error.")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	exit := 0
	results := []fileFinding{}
	for _, path := range flag.Args() {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			exit = 2
			continue
		}
		r := eiep.Check(data)
		findings := r.Findings
		if len(findings) > 0 && exit == 0 {
			exit = 1
		}
		if *asJSON {
			for _, f := range findings {
				results = append(results, fileFinding{path, f})
			}
			continue
		}
		printText(path, r, *all)
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(results)
	}
	os.Exit(exit)
}

func printText(path string, r eiep.Result, all bool) {
	findings := r.Findings
	format := ""
	if r.Format != "" {
		format = cyan(r.Format) + ": "
	}
	if len(findings) == 0 {
		head, rest, _ := strings.Cut(r.Summary, "\n")
		fmt.Printf("%s: %s%s: %s\n", bold(path), format, green("valid"), head)
		if rest != "" {
			fmt.Println(paintLines(rest, registerLine))
		}
		return
	}
	counts := map[kind]int{}
	last := map[kind]int{}
	var order []kind
	for _, f := range findings {
		k := kind{f.Field, f.Code}
		if counts[k] == 0 {
			order = append(order, k)
		}
		counts[k]++
		last[k] = f.Line
		if !all && counts[k] > perKind {
			continue
		}
		loc := bold(path) + cyan(fmt.Sprintf(":%d", f.Line))
		if f.Field != "" {
			fmt.Printf("%s: %s: %s\n", loc, yellow(f.Field), f.Msg)
		} else {
			fmt.Printf("%s: %s\n", loc, f.Msg)
		}
	}
	for _, k := range order {
		if n := counts[k] - perKind; !all && n > 0 {
			name := k.field
			if name == "" {
				name = k.code
			}
			fmt.Printf("%s: %s\n", bold(path), dim(fmt.Sprintf("...and %d more %q problems like these, up to line %d (use --all to list them)", n, name, last[k])))
		}
	}
	fmt.Printf("%s: %s%s\n", bold(path), format, red(fmt.Sprintf("%d problems", len(findings))))
}
