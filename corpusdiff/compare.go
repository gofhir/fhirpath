package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// compareCommand reads two evaluations of the same corpus and reports the
// answers that differ, grouped by expression, most frequent first, with one
// example of each. Every difference is worth reading: most turn out to be
// corrections, and the one that is not is the reason to run this.
func compareCommand(args []string) error {
	flags := flag.NewFlagSet("compare", flag.ExitOnError)
	verbose := flags.Bool("v", false, "list every differing evaluation, not one per expression")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		usage()
	}

	base, err := load(flags.Arg(0))
	if err != nil {
		return err
	}
	head, err := load(flags.Arg(1))
	if err != nil {
		return err
	}

	byExpr, total := differences(base, head)
	exprs := make([]string, 0, len(byExpr))
	for e := range byExpr {
		exprs = append(exprs, e)
	}
	sort.Slice(exprs, func(i, j int) bool {
		if len(byExpr[exprs[i]]) != len(byExpr[exprs[j]]) {
			return len(byExpr[exprs[i]]) > len(byExpr[exprs[j]])
		}
		return exprs[i] < exprs[j]
	})

	fmt.Printf("%d evaluations, %d answers differ across %d expressions\n", len(base), total, len(exprs))
	if onlyBase, onlyHead := unmatched(base, head), unmatched(head, base); onlyBase > 0 || onlyHead > 0 {
		fmt.Printf("(%d evaluations only in the base, %d only in the head)\n", onlyBase, onlyHead)
	}

	for _, e := range exprs {
		files := byExpr[e]
		fmt.Printf("\n%5d  %s\n", len(files), e)
		shown := files[:1]
		if *verbose {
			shown = files
		}
		for _, file := range shown {
			key := evaluation{file, e}
			fmt.Printf("       %s\n         base: %s\n         head: %s\n", file, display(base[key]), display(head[key]))
		}
	}
	return nil
}

// differences groups the evaluations whose answers differ by expression, each
// with its files in order, and counts them.
func differences(base, head map[evaluation]string) (byExpr map[string][]string, total int) {
	byExpr = map[string][]string{}
	for key, was := range base {
		if is, ok := head[key]; ok && is != was {
			byExpr[key.expr] = append(byExpr[key.expr], key.file)
			total++
		}
	}
	for _, files := range byExpr {
		sort.Strings(files)
	}
	return byExpr, total
}

// unmatched counts the evaluations of one run that the other does not have.
func unmatched(of, in map[evaluation]string) int {
	n := 0
	for key := range of {
		if _, ok := in[key]; !ok {
			n++
		}
	}
	return n
}

// display shortens an answer for reading: the raw and Document answers, each
// unquoted, with runs of whitespace closed up, and cut at a line's width. The
// full answers are in the files compared.
func display(answer string) string {
	parts := strings.Split(answer, "\t")
	for i, part := range parts {
		if unquoted, err := strconv.Unquote(part); err == nil {
			part = unquoted
		}
		part = strings.Join(strings.Fields(part), " ")
		if len(part) > 100 {
			part = part[:100] + "…"
		}
		parts[i] = part
	}
	if len(parts) == 2 && parts[0] == parts[1] {
		return parts[0]
	}
	return strings.Join(parts, "   (Document: ") + strings.Repeat(")", len(parts)-1)
}

type evaluation struct{ file, expr string }

func load(path string) (map[evaluation]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	answers := map[evaluation]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		fields := strings.SplitN(sc.Text(), "\t", 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf("%s: malformed line %q", path, sc.Text())
		}
		answers[evaluation{fields[0], fields[1]}] = fields[2]
	}
	return answers, sc.Err()
}
