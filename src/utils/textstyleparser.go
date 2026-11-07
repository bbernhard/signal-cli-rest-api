package utils

// Converts between the markdown-like markup accepted by text_mode 'styled'
// and signal-cli's text styles (plain text plus start:length:STYLE ranges,
// with start and length counted in UTF-16 code units).
//
// Markup:
//
//	*italic*  **bold**  ***bold italic***  ~strikethrough~ (or ~~strikethrough~~)
//	||spoiler||  `monospace`
//
// Styles can be nested and can follow each other without spaces in between
// (e.g. "**bold***italic*"). Asterisks are matched with CommonMark's emphasis
// rules (https://spec.commonmark.org/0.31.2/#emphasis-and-strong-emphasis),
// minus its punctuation special cases: a marker can open a style if it isn't
// followed by whitespace, and close one if it isn't preceded by whitespace.
// Markers that don't match up are kept as literal text, so "5 * 3" stays as is.
// The text of a monospace span is taken literally.
//
// A backslash escapes one marker character, as in CommonMark, so a literal "**"
// is written "\*\*". A backslash in front of any other character is kept as is,
// which means paths like C:\Users are left alone. To get a literal backslash in
// front of a marker, double it: "\\*".

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
)

const (
	Normal        string = "NORMAL"
	Bold                 = "BOLD"
	Italic               = "ITALIC"
	Monospace            = "MONOSPACE"
	Strikethrough        = "STRIKETHROUGH"
	Spoiler              = "SPOILER"
)

const EscapeCharacter rune = '\\'

// start and length are in UTF-16 code units
type TextStyleRange struct {
	Start  int    `json:"start"`
	Length int    `json:"length"`
	Style  string `json:"style"`
}

func (s TextStyleRange) String() string {
	return strconv.Itoa(s.Start) + ":" + strconv.Itoa(s.Length) + ":" + s.Style
}

func isMarker(r rune) bool {
	return r == '*' || r == '~' || r == '|' || r == '`'
}

func getUtf16StringLength(s string) int {
	length := 0
	for _, r := range s {
		length += utf16.RuneLen(r)
	}
	return length
}

func appendBackslashes(buf []rune, n int) []rune {
	for i := 0; i < n; i++ {
		buf = append(buf, EscapeCharacter)
	}
	return buf
}

type TextstyleParser struct {
	input string
}

func NewTextstyleParser(input string) *TextstyleParser {
	return &TextstyleParser{input: input}
}

func (l *TextstyleParser) Parse() (string, []string) {
	text, styles := parseTextStyles(l.input)
	formatStrings := make([]string, 0, len(styles))
	for _, style := range styles {
		formatStrings = append(formatStrings, style.String())
	}
	return text, formatStrings
}

type mdNode struct {
	text []rune
	mono bool

	delim     rune
	origLen   int // needed for the rule of 3
	remaining int
	canOpen   bool
	canClose  bool
	active    bool

	// leftover characters are literal and sit between the closed and the opened styles
	closePos int
	openPos  int
}

type mdMatch struct {
	opener, closer *mdNode
	style          string
}

func parseTextStyles(input string) (string, []TextStyleRange) {
	src := []rune(input)
	n := len(src)
	at := func(i int) rune { // out of range counts as whitespace
		if i < 0 || i >= n {
			return ' '
		}
		return src[i]
	}

	var nodes []*mdNode
	var buf []rune
	flush := func() {
		if len(buf) > 0 {
			nodes = append(nodes, &mdNode{text: buf})
			buf = nil
		}
	}

	for i := 0; i < n; {
		c := src[i]

		if c == EscapeCharacter {
			buf, i = readEscape(src, i, buf)
			continue
		}

		if !isMarker(c) {
			buf = append(buf, c)
			i++
			continue
		}

		if c == '`' {
			if !unicode.IsSpace(at(i + 1)) {
				if content, end, ok := scanCodeSpan(src, i+1); ok {
					flush()
					nodes = append(nodes, &mdNode{text: content, mono: true})
					i = end + 1
					continue
				}
			}
			buf = append(buf, c)
			i++
			continue
		}

		j := i
		for j < n && src[j] == c {
			j++
		}
		k := j - i
		canOpen := !unicode.IsSpace(at(j))
		canClose := !unicode.IsSpace(at(i - 1))
		valid := c == '*' || (c == '~' && k <= 2) || (c == '|' && k == 2)
		if !valid || (!canOpen && !canClose) {
			buf = append(buf, src[i:j]...)
		} else {
			flush()
			nodes = append(nodes, &mdNode{delim: c, origLen: k, remaining: k, canOpen: canOpen, canClose: canClose, active: true})
		}
		i = j
	}
	flush()

	matches := matchDelimiters(nodes)

	var out []rune
	var ranges []TextStyleRange
	pos := 0
	for _, node := range nodes {
		if node.delim == 0 {
			length := getUtf16StringLength(string(node.text))
			if node.mono {
				ranges = append(ranges, TextStyleRange{Start: pos, Length: length, Style: Monospace})
			}
			out = append(out, node.text...)
			pos += length
			continue
		}
		node.closePos = pos
		for x := 0; x < node.remaining; x++ {
			out = append(out, node.delim)
		}
		pos += node.remaining
		node.openPos = pos
	}
	for _, m := range matches {
		ranges = append(ranges, TextStyleRange{Start: m.opener.openPos, Length: m.closer.closePos - m.opener.openPos, Style: m.style})
	}
	// for equal ends the inner style comes first
	sort.SliceStable(ranges, func(a, b int) bool {
		return ranges[a].Start+ranges[a].Length < ranges[b].Start+ranges[b].Length
	})
	result := ranges[:0]
	for _, r := range ranges {
		if r.Length > 0 {
			result = append(result, r)
		}
	}
	return string(out), result
}

// in front of a marker two backslashes give one and an odd one escapes the marker
func readEscape(src []rune, i int, buf []rune) ([]rune, int) {
	j := i
	for j < len(src) && src[j] == EscapeCharacter {
		j++
	}
	k := j - i
	if j == len(src) || !isMarker(src[j]) {
		return appendBackslashes(buf, k), j
	}
	buf = appendBackslashes(buf, k/2)
	if k%2 == 0 {
		return buf, j
	}
	return append(buf, src[j]), j + 1
}

// inside a code span only escapes are processed
func scanCodeSpan(src []rune, start int) ([]rune, int, bool) {
	var content []rune
	for p := start; p < len(src); {
		c := src[p]
		if c == EscapeCharacter {
			content, p = readEscape(src, p, content)
			continue
		}
		if c == '`' && len(content) > 0 && !unicode.IsSpace(src[p-1]) {
			return content, p, true
		}
		content = append(content, c)
		p++
	}
	return nil, 0, false
}

// see CommonMark's "process emphasis" procedure
func matchDelimiters(nodes []*mdNode) []mdMatch {
	var delims []*mdNode
	for _, node := range nodes {
		if node.delim != 0 {
			delims = append(delims, node)
		}
	}

	var matches []mdMatch
	for ci, closer := range delims {
		if !closer.canClose {
			continue
		}
		for closer.remaining > 0 {
			oi := findOpener(delims, ci)
			if oi < 0 {
				break
			}
			opener := delims[oi]

			use := closer.remaining // '~' and '|' runs are used up as a whole
			var style string
			switch closer.delim {
			case '*':
				use = 1
				style = Italic
				if closer.remaining >= 2 && opener.remaining >= 2 {
					use = 2
					style = Bold
				}
			case '~':
				style = Strikethrough
			case '|':
				style = Spoiler
			}
			opener.remaining -= use
			closer.remaining -= use
			matches = append(matches, mdMatch{opener: opener, closer: closer, style: style})

			// unmatched markers between the opener and the closer become literal text
			for k := oi + 1; k < ci; k++ {
				delims[k].active = false
			}
		}
	}
	return matches
}

func findOpener(delims []*mdNode, ci int) int {
	closer := delims[ci]
	for oi := ci - 1; oi >= 0; oi-- {
		opener := delims[oi]
		if !opener.active || !opener.canOpen || opener.delim != closer.delim || opener.remaining == 0 {
			continue
		}
		switch closer.delim {
		case '*':
			// CommonMark's rule of 3
			if (closer.canOpen || opener.canClose) && closer.origLen%3 != 0 && (opener.origLen+closer.origLen)%3 == 0 {
				continue
			}
		case '~':
			if opener.remaining != closer.remaining {
				continue
			}
		}
		return oi
	}
	return -1
}

// monospace has to be innermost, as markers inside a code span are literal
const (
	spoilerBit = iota
	strikethroughBit
	boldBit
	italicBit
	monoBit
)

var styleBits = [...]struct {
	style  string
	marker string
}{
	spoilerBit:       {Spoiler, "||"},
	strikethroughBit: {Strikethrough, "~"},
	boldBit:          {Bold, "**"},
	italicBit:        {Italic, "*"},
	monoBit:          {Monospace, "`"},
}

// bold and italic share the asterisk, so they are tried last
var fallbackBitOrder = []int{spoilerBit, strikethroughBit, monoBit, boldBit, italicBit}

// the styles of a received message are chosen by its sender, so the work is bounded
const maxFallbackAttempts = 32

func styleBit(style string) int {
	for bit, s := range styleBits {
		if s.style == style {
			return bit
		}
	}
	return -1
}

func styleMasks(runes []rune, styles []TextStyleRange) []uint8 {
	offsets := make([]int, len(runes)+1)
	for i, r := range runes {
		offsets[i+1] = offsets[i] + utf16.RuneLen(r)
	}
	total := offsets[len(runes)]

	masks := make([]uint8, len(runes))
	for _, s := range styles {
		bit := styleBit(s.Style)
		start := max(s.Start, 0)
		end := min(s.Start+s.Length, total)
		if bit < 0 || end <= start {
			continue
		}
		a := sort.Search(len(offsets), func(i int) bool { return offsets[i] > start }) - 1
		b := sort.Search(len(offsets), func(i int) bool { return offsets[i] >= end })
		for i := a; i < b; i++ {
			masks[i] |= 1 << bit
		}
	}
	return masks
}

// FormatTextStyles converts signal-cli text styles into the markup of text_mode 'styled'.
// Styles on whitespace and some bold/italic overlaps within a word can't be written and are dropped.
func FormatTextStyles(message string, styles []TextStyleRange) string {
	runes := []rune(message)
	masks := styleMasks(runes, styles)

	if !slices.ContainsFunc(masks, func(mask uint8) bool { return mask != 0 }) {
		if !strings.ContainsAny(message, "*~|`\\") {
			return message
		}
		return renderMarkup(runes, masks, true, false)
	}

	if markup, ok := formatVerified(runes, masks); ok {
		return markup
	}

	// fall back to adding the styles span by span
	kept := make([]uint8, len(runes))
	best := renderMarkup(runes, kept, true, false)
	attempts := 0
	for _, bit := range fallbackBitOrder {
		for i := 0; i < len(runes) && attempts < maxFallbackAttempts; i++ {
			if masks[i]&(1<<bit) == 0 {
				continue
			}
			j := i
			for j < len(runes) && masks[j]&(1<<bit) != 0 {
				j++
			}
			candidate := slices.Clone(kept)
			for x := i; x < j; x++ {
				candidate[x] |= 1 << bit
			}
			if markup, ok := formatVerified(runes, candidate); ok {
				kept, best = candidate, markup
			}
			attempts++
			i = j
		}
	}
	return best
}

func formatVerified(runes []rune, masks []uint8) (string, bool) {
	for _, sorted := range []bool{true, false} {
		for _, italicFirst := range []bool{false, true} {
			markup := renderMarkup(runes, masks, sorted, italicFirst)
			if roundTrips(runes, masks, markup) {
				return markup, true
			}
		}
	}
	return "", false
}

func roundTrips(runes []rune, masks []uint8, markup string) bool {
	text, styles := parseTextStyles(markup)
	if text != string(runes) {
		return false
	}
	got := styleMasks(runes, styles)
	for i, r := range runes {
		if !unicode.IsSpace(r) && got[i] != masks[i] {
			return false
		}
	}
	return true
}

type mdToken struct {
	delim string
	r     rune
	mono  bool
}

// markers never touch whitespace: closing ones go before it, opening ones after it.
// sorted nests the styles by where they end, italicFirst swaps bold and italic on ties.
func renderMarkup(runes []rune, masks []uint8, sorted bool, italicFirst bool) string {
	n := len(runes)
	isWord := func(i int) bool { return !unicode.IsSpace(runes[i]) }

	// end[bit][i]: end of the span containing word character i
	end := make([][]int, len(styleBits))
	for bit := range styleBits {
		end[bit] = make([]int, n)
		next := -1
		for i := n - 1; i >= 0; i-- {
			if !isWord(i) {
				continue
			}
			if masks[i]&(1<<bit) == 0 {
				next = -1
				continue
			}
			if next < 0 {
				next = i + 1
			}
			end[bit][i] = next
		}
	}

	order := func(i int, mask uint8) []int {
		var bits []int
		for bit := range styleBits {
			if mask&(1<<bit) != 0 {
				bits = append(bits, bit)
			}
		}
		rank := func(bit int) int {
			if italicFirst && bit == boldBit {
				return italicBit
			} else if italicFirst && bit == italicBit {
				return boldBit
			}
			return bit
		}
		sort.SliceStable(bits, func(a, b int) bool {
			x, y := bits[a], bits[b]
			if (x == monoBit) != (y == monoBit) {
				return y == monoBit
			}
			if end[x][i] != end[y][i] {
				return end[x][i] > end[y][i]
			}
			return rank(x) < rank(y)
		})
		return bits
	}

	var tokens []mdToken
	var stack []int
	stackMask := func() uint8 {
		var m uint8
		for _, bit := range stack {
			m |= 1 << bit
		}
		return m
	}
	pop := func() int {
		bit := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		tokens = append(tokens, mdToken{delim: styleBits[bit].marker})
		return bit
	}
	push := func(bit int) {
		stack = append(stack, bit)
		tokens = append(tokens, mdToken{delim: styleBits[bit].marker})
	}

	// returns the styles to open after the whitespace
	closeFor := func(i int, mask uint8) []int {
		if sorted {
			desired := order(i, mask)
			common := 0
			for common < len(stack) && common < len(desired) && stack[common] == desired[common] {
				common++
			}
			for len(stack) > common {
				pop()
			}
			return desired[common:]
		}
		var reopen uint8
		for stackMask()&^mask != 0 {
			if bit := pop(); mask&(1<<bit) != 0 {
				reopen |= 1 << bit
			}
		}
		opening := mask&^stackMask() | reopen
		if opening != 0 && len(stack) > 0 && stack[len(stack)-1] == monoBit {
			pop()
			opening |= 1 << monoBit
		}
		return order(i, opening)
	}

	spaceStart := 0
	var prevMask uint8
	for i := 0; i < n; i++ {
		if !isWord(i) {
			continue
		}
		var toOpen []int
		if masks[i] != prevMask { // within a span nothing changes
			toOpen = closeFor(i, masks[i])
			prevMask = masks[i]
		}
		for _, r := range runes[spaceStart:i] {
			tokens = append(tokens, mdToken{r: r})
		}
		for _, bit := range toOpen {
			push(bit)
		}
		tokens = append(tokens, mdToken{r: runes[i], mono: masks[i]&(1<<monoBit) != 0})
		spaceStart = i + 1
	}
	for len(stack) > 0 {
		pop()
	}
	for _, r := range runes[spaceStart:] {
		tokens = append(tokens, mdToken{r: r})
	}

	var sb strings.Builder
	for t := 0; t < len(tokens); t++ {
		tok := tokens[t]
		if tok.delim != "" {
			sb.WriteString(tok.delim)
			continue
		}
		if tok.r == EscapeCharacter {
			// doubled in front of a marker
			u := t
			for u < len(tokens) && tokens[u].delim == "" && tokens[u].r == EscapeCharacter {
				u++
			}
			k := u - t
			if u < len(tokens) && (tokens[u].delim != "" || isMarker(tokens[u].r)) {
				k *= 2
			}
			sb.WriteString(strings.Repeat(string(EscapeCharacter), k))
			t = u - 1
			continue
		}
		if isMarker(tok.r) && (!tok.mono || tok.r == '`') {
			sb.WriteRune(EscapeCharacter)
		}
		sb.WriteRune(tok.r)
	}
	return sb.String()
}
