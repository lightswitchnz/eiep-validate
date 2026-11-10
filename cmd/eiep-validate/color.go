package main

import (
	"os"
	"strings"
)

var useColor = os.Getenv("NO_COLOR") == "" && isTerminal() && enableVT()

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func paint(code, s string) string {
	if !useColor || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func bold(s string) string   { return paint("1", s) }
func dim(s string) string    { return paint("2", s) }
func red(s string) string    { return paint("31", s) }
func green(s string) string  { return paint("32", s) }
func yellow(s string) string { return paint("33", s) }
func cyan(s string) string   { return paint("36", s) }
func blue(s string) string   { return paint("34", s) }

var flowColor = map[string]func(string) string{"X (IN)": blue, "I (OUT)": green}

// registerLine dims a summary register line but colours its flow.
func registerLine(s string) string {
	for flow, color := range flowColor {
		if before, after, ok := strings.Cut(s, "flow "+flow); ok {
			return dim(before) + color("flow "+flow) + dim(after)
		}
	}
	return dim(s)
}

func paintLines(s string, f func(string) string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = f(lines[i])
	}
	return strings.Join(lines, "\n")
}
