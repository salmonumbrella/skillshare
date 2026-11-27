package contextrouter

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The patterns the reference matches where regionIndex answers from its tables.
var (
	referenceInlineEndRe  = regexp.MustCompile(`^(?:` + pySpace + `+` + titlePattern + `)?` + pySpace + `*\)`)
	referenceInlineHTMLRe = regexp.MustCompile(`^(?:<!--[\s\S]*?-->|<\?[\s\S]*?\?>|<![A-Z][\s\S]*?>|<!\[CDATA\[[\s\S]*?\]\]>|` + htmlTagPattern + `)`)
)

// The tables give the answers the reference's rescans give, at every position
// the inline scanner can ask from.
func TestRegionIndexMatchesRescans(t *testing.T) {
	tokens := []string{"(", ")", "(", ")", "\\", "\\\\", " ", "\t", "\n", "\"", "'", "<", ">", "a", "é", " ",
		"](", "<!--", "-->", "<?", "?>", "<!A", "<![CDATA[", "]]>", "<a b='c'>", "</a>", "x y"}
	rng := rand.New(rand.NewSource(19))
	asked, rounds := 0, 20000
	if raceBuild {
		rounds = 4000
	}
	for round := 0; round < rounds; round++ {
		var b strings.Builder
		for n := rng.Intn(24); n > 0; n-- {
			b.WriteString(tokens[rng.Intn(len(tokens))])
		}
		text := "pad " + b.String() + " pad"
		begin, end := 2, len(text)-2 // a region inside the text, as a paragraph is
		index := regionIndex{text: text, begin: begin, end: end}
		for p := begin; p <= end; p++ {
			// A destination starts after "](": the scanner never asks from anywhere else.
			if p-2 >= begin && text[p-2:p] == "](" {
				_, wantStop, wantOK := destination(text, p, end)
				if stop, ok := index.destination(p); ok != wantOK || (ok && stop != wantStop) {
					t.Fatalf("destination(%q, %d): got %d %v, want %d %v", text[begin:end], p-begin, stop-begin, ok, wantStop-begin, wantOK)
				}
				asked++
			}
			want := referenceInlineEndRe.FindStringIndex(text[p:end])
			if stop, ok := index.inlineEnd(p); ok != (want != nil) || (ok && stop != p+want[1]) {
				t.Fatalf("inlineEnd(%q, %d): got %d %v, want %v", text[begin:end], p-begin, stop-begin, ok, want)
			}
			if p < end && text[p] == '<' {
				want := referenceInlineHTMLRe.FindStringIndex(text[p:end])
				if stop, ok := index.inlineHTML(p); ok != (want != nil) || (ok && stop != p+want[1]) {
					t.Fatalf("inlineHTML(%q, %d): got %d %v, want %v", text[begin:end], p-begin, stop-begin, ok, want)
				}
			}
		}
	}
	if asked < rounds/4 {
		t.Fatalf("only %d destinations compared", asked)
	}
}

// hostileBodies are n bytes each of the shapes that made the scanner, or the
// reference it mirrors, quadratic: every link rescanning the rest of the page,
// every list marker re-reading its line, every line walking deep nesting.
func hostileBodies(n int) map[string]string {
	fill := func(unit string) string { return strings.Repeat(unit, n/len(unit)+1)[:n] }
	half := n / 2
	return map[string]string{
		"unclosed links":           fill("[a]("),
		"unclosed parentheses":     fill("[a](("),
		"unclosed paren titles":    fill("[a](x ("),
		"unclosed quoted titles":   fill("[a](x \""),
		"list markers on one line": strings.Repeat("- ", half) + "x",
		"blank lines under lists":  strings.Repeat("- ", half/2) + "x" + strings.Repeat("\n", half),
		"quotes under lists":       strings.Repeat("- ", half/2) + "x" + strings.Repeat("\n>", half/2),
		"lists inside a quote":     "> " + strings.Repeat("- ", half/2) + "x" + strings.Repeat("\n>", half/2),
		"nested brackets":          strings.Repeat("[", half) + strings.Repeat("]", half),
		"links inside image text":  strings.Repeat("![", half/2) + fill("[docs/run.md](../docs/run.md)")[:half],
		"unclosed comments":        fill("<!--"),
		"unclosed declarations":    fill("<!A"),
		"backticks":                fill("`a"),
		"many definitions":         fill("[r]: x\n"),
		"many real links":          fill("[docs/run.md](../docs/run.md)\n"),
	}
}

// Four times the input must cost about four times the time. Quadratic work
// costs sixteen times, whatever the machine or the build: an absolute bound
// would have to be tuned for the race detector and for a loaded CI runner.
func TestCompactScalesLinearlyOnHostileInput(t *testing.T) {
	small, large := 128<<10, 512<<10
	if raceBuild {
		small, large = 4<<10, 16<<10 // keeps the whole test to a couple of seconds there
	}
	timed := func(body string) time.Duration {
		start := time.Now()
		if _, ok := compactPathLinks(body, "wiki/guide.md", nil); !ok {
			t.Fatal("stopped without a stopper")
		}
		return time.Since(start)
	}
	// The fastest of several interleaved runs stands for each size: other packages'
	// tests share the machine, and a busy moment in one run must not read as growth.
	ratio := func(name string, runs int) (base, grown time.Duration, r float64) {
		smallBody, largeBody := hostileBodies(small)[name], hostileBodies(large)[name]
		base, grown = time.Hour, time.Hour
		for i := 0; i < runs; i++ {
			base, grown = min(base, timed(smallBody)), min(grown, timed(largeBody))
		}
		return base, grown, float64(grown) / float64(max(base, time.Microsecond))
	}
	for name := range hostileBodies(16) {
		base, grown, r := ratio(name, 2)
		if r > 10 {
			base, grown, r = ratio(name, 6) // look again before calling it growth
		}
		if r > 10 {
			t.Errorf("%s: %v for %d bytes, %v for %d bytes: %.1f times the time for 4 times the input", name, base, small, grown, large, r)
		}
	}
}

// The largest file a report reads scans in a fraction of a second. Not under
// the race detector, whose slowdown says nothing about the scanner.
func TestCompactScansAMegabyteQuickly(t *testing.T) {
	if raceBuild || testing.Short() {
		t.Skip("an absolute time bound for an ordinary build")
	}
	const size = 1 << 20
	for name, body := range hostileBodies(size) {
		start := time.Now()
		if _, ok := compactPathLinks(body, "wiki/guide.md", nil); !ok {
			t.Errorf("%s: stopped without a stopper", name)
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("%s: %v for %d bytes", name, elapsed, size)
		}
	}
}

func TestStopperHaltsAScan(t *testing.T) {
	body := strings.Repeat("[docs/run.md](../docs/run.md)\n", 2000)
	if _, ok := compactPathLinks(body, "wiki/guide.md", &stopper{deadline: time.Now().Add(-time.Second)}); ok {
		t.Fatal("a scan past its deadline ran to the end")
	}
	if out, ok := compactPathLinks(body, "wiki/guide.md", &stopper{deadline: time.Now().Add(time.Hour)}); !ok || !strings.HasPrefix(out, "`docs/run.md`\n") {
		t.Fatal("a scan within its deadline did not finish")
	}
}
