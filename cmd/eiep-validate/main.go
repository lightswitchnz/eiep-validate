package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lightswitchnz/eiep-validate/eiep"
)

// perKind is how many findings of one kind are printed before the rest are summarised.
const perKind = 3

type fileFinding struct {
	File string `json:"file"`
	eiep.Finding
	Warning bool `json:"warning,omitempty"`
}

// strictOnly are warnings shown only with --strict: against the spec but harmless to a reader of the file.
var strictOnly = map[string]bool{"too_long": true, "shared_description": true, "duplicate_description": true}

type kind struct{ field, code string }

type filePlan struct {
	File string `json:"file"`
	eiep.Plan
}

func main() {
	asJSON := flag.Bool("json", false, "print every finding as JSON")
	all := flag.Bool("all", false, "print every finding instead of summarising repeats")
	plans := flag.Bool("plans", false, "list each EIEP14A plan with the attributes that apply to it, such as requirements and perks")
	strict := flag.Bool("strict", false, "also warn about values longer than the spec allows and plans sharing a description")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: eiep-validate [--json] [--all] [--strict] [--plans] [FILE...]")
		fmt.Fprintln(os.Stderr, "Validates EIEP files strictly against the Electricity Authority spec.")
		fmt.Fprintln(os.Stderr, "With no FILE, opens a web UI in the browser.")
		fmt.Fprintln(os.Stderr, "Exit status: 0 valid (warnings allowed), 1 findings, 2 usage or read error.")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		serve()
	}

	exit := 0
	results := []fileFinding{}
	planResults := []filePlan{}
	for _, path := range flag.Args() {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			exit = 2
			continue
		}
		r := check(data, *strict)
		findings := r.Findings
		if len(findings) > 0 && exit == 0 {
			exit = 1
		}
		if *plans {
			if *asJSON {
				for _, p := range r.Plans() {
					planResults = append(planResults, filePlan{path, p})
				}
			} else {
				printPlans(filepath.Base(path), r.Plans())
			}
			continue
		}
		if *asJSON {
			for _, f := range findings {
				results = append(results, fileFinding{path, f, false})
			}
			for _, f := range r.Warnings {
				results = append(results, fileFinding{path, f, true})
			}
			continue
		}
		printText(filepath.Base(path), r, *all)
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if *plans {
			enc.Encode(planResults)
		} else {
			enc.Encode(results)
		}
	}
	os.Exit(exit)
}

func check(data []byte, strict bool) eiep.Result {
	r := eiep.Check(data)
	if !strict {
		r.Warnings = slices.DeleteFunc(r.Warnings, func(f eiep.Finding) bool { return strictOnly[f.Code] })
	}
	return r
}

func printText(path string, r eiep.Result, all bool) {
	format := ""
	if r.Format != "" {
		format = cyan(r.Format) + ": "
	}
	printFindings(path, r.Warnings, all, yellow("warning")+": ", "warnings")
	warned := ""
	if n := len(r.Warnings); n > 0 {
		warned = yellow(fmt.Sprintf(" (%d warnings)", n))
	}
	if len(r.Findings) == 0 {
		head, rest, _ := strings.Cut(r.Summary, "\n")
		fmt.Printf("%s: %s%s%s: %s\n", bold(path), format, green("valid"), warned, head)
		if rest != "" {
			fmt.Println(paintLines(rest, registerLine))
		}
		return
	}
	printFindings(path, r.Findings, all, "", "problems")
	fmt.Printf("%s: %s%s%s\n", bold(path), format, red(fmt.Sprintf("%d problems", len(r.Findings))), warned)
}

func printFindings(path string, findings []eiep.Finding, all bool, label, noun string) {
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
			fmt.Printf("%s: %s%s: %s\n", loc, label, yellow(f.Field), f.Msg)
		} else {
			fmt.Printf("%s: %s%s\n", loc, label, f.Msg)
		}
	}
	for _, k := range order {
		if n := counts[k] - perKind; !all && n > 0 {
			name := k.field
			if name == "" {
				name = k.code
			}
			fmt.Printf("%s: %s\n", bold(path), dim(fmt.Sprintf("...and %d more %q %s like these, up to line %d (use --all to list them)", n, name, noun, last[k])))
		}
	}
}

func printPlans(path string, plans []eiep.Plan) {
	if plans == nil {
		fmt.Printf("%s: %s\n", bold(path), dim("no EIEP14A plans"))
	}
	for _, p := range plans {
		fmt.Printf("%s: %s / %s / %s %s\n", bold(path)+cyan(fmt.Sprintf(":%d", p.Line)), p.Retailer, p.Customer, bold(p.Description), dim(p.ID))
		for _, a := range p.Attributes {
			code := a.Code
			if a.Value != "" {
				code += " " + a.Value
			}
			fmt.Printf("  %s %s %s %s\n", yellow(a.Kind+":"), code, dim("("+a.Level+")"), a.Description)
		}
	}
}
