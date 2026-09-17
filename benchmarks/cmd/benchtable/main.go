// Command benchtable turns benchmark results into the performance tables of
// the README:
//
//	go run ./cmd/benchtable [-readme ../README.md] results/decimal.txt results/compare.txt
//
// It reads the output of go test -bench, which may hold several runs of each
// benchmark, and reports the median time of each. With -readme it replaces
// the text between the lines "<!-- benchmarks -->" and "<!-- /benchmarks -->"
// of that file; otherwise it prints the tables.
//
// Results that vary too much between runs are not worth publishing. If more
// than a tenth of the benchmarks are noisy, benchtable lists them and exits
// without updating the file, unless -force is given.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	beginMarker = "<!-- benchmarks -->"
	endMarker   = "<!-- /benchmarks -->"
)

type result struct {
	ns     []float64
	allocs float64
}

type results struct {
	config map[string]string
	byName map[string]*result
}

func main() {
	log.SetFlags(0)
	readme := flag.String("readme", "", "update the tables in this `file`")
	force := flag.Bool("force", false, "publish the results even if they are noisy")
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatal("usage: benchtable [-readme file] [-force] results...")
	}
	rs := results{config: map[string]string{}, byName: map[string]*result{}}
	for _, name := range flag.Args() {
		if err := rs.read(name); err != nil {
			log.Fatal(err)
		}
	}
	if noisy := rs.noisy(); len(noisy) > len(rs.byName)/10 {
		for _, s := range noisy {
			log.Print(s)
		}
		msg := fmt.Sprintf("%d of %d benchmarks vary by more than %.0f%% between runs; "+
			"is the machine busy, or the process running at background priority?",
			len(noisy), len(rs.byName), maxSpread*100)
		if !*force {
			log.Fatal(msg + " (use -force to publish anyway)")
		}
		log.Print(msg)
	}
	var b bytes.Buffer
	rs.write(&b)
	if *readme == "" {
		os.Stdout.Write(b.Bytes())
		return
	}
	if err := replaceBetweenMarkers(*readme, b.Bytes()); err != nil {
		log.Fatal(err)
	}
}

var procsSuffix = regexp.MustCompile(`-\d+$`)

// read adds the results in the named file.
func (rs *results) read(name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		switch {
		case len(fields) >= 4 && strings.HasPrefix(fields[0], "Benchmark"):
			key := procsSuffix.ReplaceAllString(strings.TrimPrefix(fields[0], "Benchmark"), "")
			r := rs.byName[key]
			if r == nil {
				r = &result{}
				rs.byName[key] = r
			}
			for i := 2; i+1 < len(fields); i += 2 {
				v, err := strconv.ParseFloat(fields[i], 64)
				if err != nil {
					return fmt.Errorf("%s: %q: %v", name, line, err)
				}
				switch fields[i+1] {
				case "ns/op":
					r.ns = append(r.ns, v)
				case "allocs/op":
					r.allocs = v
				}
			}
		case len(fields) >= 2 && strings.HasSuffix(fields[0], ":"):
			key := strings.TrimSuffix(fields[0], ":")
			if _, ok := rs.config[key]; !ok {
				rs.config[key] = strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
			}
		}
	}
	return sc.Err()
}

// maxSpread is the largest variation between runs, relative to the median,
// that noisy tolerates.
const maxSpread = 0.15

// median returns the median time of a benchmark, and the range of the times
// of its runs other than the fastest and the slowest, relative to the median.
func (r *result) median() (m, spread float64) {
	ns := slices.Clone(r.ns)
	slices.Sort(ns)
	m = ns[len(ns)/2]
	if len(ns)%2 == 0 {
		m = (ns[len(ns)/2-1] + ns[len(ns)/2]) / 2
	}
	if len(ns) >= 4 {
		ns = ns[1 : len(ns)-1]
	}
	return m, (ns[len(ns)-1] - ns[0]) / m
}

// noisy describes the benchmarks whose runs vary by more than maxSpread.
func (rs *results) noisy() []string {
	var out []string
	for name, r := range rs.byName {
		if len(r.ns) < 2 {
			continue
		}
		if m, spread := r.median(); spread > maxSpread {
			out = append(out, fmt.Sprintf("%s: median %.4gns, spread %.0f%%", name, m, spread*100))
		}
	}
	slices.Sort(out)
	return out
}

// cell formats the median time of a benchmark, with its allocations.
func (rs *results) cell(name string) string {
	r := rs.byName[name]
	if r == nil || len(r.ns) == 0 {
		return "—"
	}
	m, _ := r.median()
	var s string
	switch {
	case m < 10:
		s = strconv.FormatFloat(m, 'f', 1, 64)
	default:
		s = thousands(int64(m + 0.5))
	}
	if r.allocs > 0 {
		s += fmt.Sprintf(" (%d)", int64(r.allocs))
	}
	return s
}

func thousands(v int64) string {
	s := strconv.FormatInt(v, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// runs returns the smallest number of runs of any benchmark.
func (rs *results) runs() int {
	runs := 0
	for _, r := range rs.byName {
		if runs == 0 || len(r.ns) < runs {
			runs = len(r.ns)
		}
	}
	return runs
}

var operations = []struct{ name, label string }{
	{"Add", "`Add`"},
	{"Sub", "`Sub`"},
	{"Mul", "`Mul`"},
	{"Quo", "`Quo`"},
	{"FMA", "`FMA`"},
	{"Sqrt", "`Sqrt`"},
	{"Remainder", "`Remainder`"},
	{"Quantize", "`Quantize` to 0.01"},
	{"Round", "`Round(2)`"},
	{"Cmp", "`Cmp`"},
	{"Parse", "`Parse`"},
	{"String", "`String`"},
	{"AppendText", "`AppendText`"},
	{"Int64", "`Int64`"},
	{"Float64", "`Float64`"},
	{"FromFloat", "from `float64`"},
}

var workloads = []struct {
	name, title, tendex string
	libraries           []string
}{
	{"Amounts", "Everyday amounts, up to 9 digits", "Decimal64",
		[]string{"tendex", "float64", "anz", "govalues", "udecimal", "shopspring", "apd", "ericlagergren"}},
	{"Digits16", "16 significant digits", "Decimal64",
		[]string{"tendex", "float64", "anz", "govalues", "udecimal", "shopspring", "apd", "ericlagergren"}},
	{"Digits34", "34 significant digits", "Decimal128",
		[]string{"tendex", "woodsbury", "shopspring", "apd", "ericlagergren"}},
}

var libraryLabels = map[string]string{
	"float64":       "`float64` (binary)",
	"anz":           "[anz-bank/decimal](https://github.com/anz-bank/decimal)",
	"govalues":      "[govalues/decimal](https://github.com/govalues/decimal)",
	"udecimal":      "[quagmt/udecimal](https://github.com/quagmt/udecimal)",
	"shopspring":    "[shopspring/decimal](https://github.com/shopspring/decimal)",
	"apd":           "[cockroachdb/apd](https://github.com/cockroachdb/apd)",
	"ericlagergren": "[ericlagergren/decimal](https://github.com/ericlagergren/decimal)",
	"woodsbury":     "[woodsbury/decimal128](https://github.com/woodsbury/decimal128)",
}

// table renders rows as a Markdown table, the first row being the header. The
// columns are padded to a constant width, and every column but the first is
// aligned right, which is how the repository's Markdown formatter leaves a
// table: writing them that way here keeps regenerated tables free of changes
// that are only whitespace.
func table(b *bytes.Buffer, rows [][]string) {
	width := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			width[i] = max(width[i], len([]rune(cell)))
		}
	}
	for i, row := range rows {
		for j, cell := range row {
			pad := strings.Repeat(" ", width[j]-len([]rune(cell)))
			if j == 0 {
				fmt.Fprintf(b, "| %s%s ", cell, pad)
			} else {
				fmt.Fprintf(b, "| %s%s ", pad, cell)
			}
		}
		b.WriteString("|\n")
		if i == 0 {
			for j := range row {
				dashes := strings.Repeat("-", width[j]+2)
				if j > 0 {
					dashes = dashes[:len(dashes)-1] + ":"
				}
				fmt.Fprintf(b, "|%s", dashes)
			}
			b.WriteString("|\n")
		}
	}
}

// write writes the tables in Markdown.
func (rs *results) write(b *bytes.Buffer) {
	c := rs.config
	fmt.Fprintf(b, "Measured on %s (%s/%s) with %s: the median of %d runs, each of at least %s.\n",
		c["cpu"], c["goos"], c["goarch"], c["go"], rs.runs(), c["benchtime"])
	b.WriteString("Times are in nanoseconds per operation; allocations per operation follow in\nparentheses where there are any.\n\n")

	b.WriteString("#### This package\n\n")
	rows := [][]string{{"Operation"}}
	for _, format := range []string{"Decimal32", "Decimal64", "Decimal128"} {
		for _, size := range []string{"short", "full"} {
			rows[0] = append(rows[0], "`"+format+"` "+size)
		}
	}
	for _, op := range operations {
		row := []string{op.label}
		for _, format := range []string{"Decimal32", "Decimal64", "Decimal128"} {
			for _, size := range []string{"Short", "Full"} {
				row = append(row, rs.cell(format+"/"+op.name+"/"+size))
			}
		}
		rows = append(rows, row)
	}
	table(b, rows)

	b.WriteString("\n#### Other libraries\n")
	for _, w := range workloads {
		fmt.Fprintf(b, "\n%s:\n\n", w.title)
		rows := [][]string{{"Library", "Add", "Mul", "Quo", "Parse", "String"}}
		for _, lib := range w.libraries {
			label := libraryLabels[lib]
			if lib == "tendex" {
				label = "**tendex/decimal** `" + w.tendex + "`"
			}
			row := []string{label}
			for _, op := range []string{"Add", "Mul", "Quo", "Parse", "String"} {
				row = append(row, rs.cell(w.name+"/"+op+"/"+lib))
			}
			rows = append(rows, row)
		}
		table(b, rows)
	}
}

// replaceBetweenMarkers replaces the lines between the markers in the named
// file with text.
func replaceBetweenMarkers(name string, text []byte) error {
	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	begin := bytes.Index(data, []byte(beginMarker+"\n"))
	end := bytes.Index(data, []byte(endMarker))
	if begin < 0 || end < begin {
		return fmt.Errorf("%s: no %s ... %s section", name, beginMarker, endMarker)
	}
	begin += len(beginMarker) + 1
	var out bytes.Buffer
	out.Write(data[:begin])
	out.WriteByte('\n')
	out.Write(text)
	out.WriteByte('\n')
	out.Write(data[end:])
	return os.WriteFile(name, out.Bytes(), 0o644)
}
