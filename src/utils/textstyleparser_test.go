package utils

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"unicode"
)

func expectMessageEqual(t *testing.T, message1 string, message2 string) {
	if message1 != message2 {
		t.Errorf("got %q, wanted %q", message1, message2)
	}
}

func expectFormatStringsEqual(t *testing.T, formatStrings1 []string, formatStrings2 []string) {
	if !reflect.DeepEqual(formatStrings1, formatStrings2) {
		t.Errorf("got %q, wanted %q", formatStrings1, formatStrings2)
	}
}

func TestSimpleItalicMessage(t *testing.T) {
	textstyleParser := NewTextstyleParser("*italic*")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "italic")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:6:ITALIC"})
}

func TestSimpleBoldMessage(t *testing.T) {
	textstyleParser := NewTextstyleParser("**bold**")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "bold")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:4:BOLD"})
}

func TestSimpleMessage(t *testing.T) {
	textstyleParser := NewTextstyleParser("*This is a italic message*")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "This is a italic message")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:24:ITALIC"})
}

func TestBoldAndItalicMessage(t *testing.T) {
	textstyleParser := NewTextstyleParser("This is a **bold** and *italic* message")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "This is a bold and italic message")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"10:4:BOLD", "19:6:ITALIC"})
}

func TestTwoBoldFormattedStrings(t *testing.T) {
	textstyleParser := NewTextstyleParser("This is a **bold** and another **bold** message")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "This is a bold and another bold message")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"10:4:BOLD", "27:4:BOLD"})
}

func TestStrikethrough(t *testing.T) {
	textstyleParser := NewTextstyleParser("This is a ~strikethrough~ and a **bold** message")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "This is a strikethrough and a bold message")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"10:13:STRIKETHROUGH", "30:4:BOLD"})
}

func TestMonospace(t *testing.T) {
	textstyleParser := NewTextstyleParser("This is a `monospace` and a **bold** message")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "This is a monospace and a bold message")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"10:9:MONOSPACE", "26:4:BOLD"})
}

func TestMulticharacterEmoji(t *testing.T) {
	textstyleParser := NewTextstyleParser("👋abcdefg")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "👋abcdefg")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{})
}

func TestMulticharacterEmojiWithBoldText(t *testing.T) {
	textstyleParser := NewTextstyleParser("👋**abcdefg**")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "👋abcdefg")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"2:7:BOLD"})
}

func TestMultipleMulticharacterEmoji(t *testing.T) {
	textstyleParser := NewTextstyleParser("👋🏾abcdefg")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "👋🏾abcdefg")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{})
}

func TestMultipleMulticharacterEmojiWithBoldText(t *testing.T) {
	textstyleParser := NewTextstyleParser("👋🏾**abcdefg**")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "👋🏾abcdefg")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"4:7:BOLD"})
}

func TestMulticharacterEmojiWithBoldText2(t *testing.T) {
	textstyleParser := NewTextstyleParser("Test 👦🏿 via **signal** API")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "Test 👦🏿 via signal API")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"14:6:BOLD"})
}

func TestSpoiler(t *testing.T) {
	textstyleParser := NewTextstyleParser("||this is a spoiler||")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "this is a spoiler")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:17:SPOILER"})
}

func TestSpoiler1(t *testing.T) {
	textstyleParser := NewTextstyleParser("||this is a spoiler|| and another ||spoiler||")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "this is a spoiler and another spoiler")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:17:SPOILER", "30:7:SPOILER"})
}

func TestBoldTextInsideSpoiler(t *testing.T) {
	textstyleParser := NewTextstyleParser("||**this is a bold text inside a spoiler**||")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "this is a bold text inside a spoiler")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:36:BOLD", "0:36:SPOILER"})
}

func TestEscapeAsterisks(t *testing.T) {
	textstyleParser := NewTextstyleParser("\\*escaped text\\*")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "*escaped text*")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{})
}

func TestEscapeAsterisks1(t *testing.T) {
	textstyleParser := NewTextstyleParser("\\**escaped text\\**")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "**escaped text**")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{})
}

func TestEscapeBackticks(t *testing.T) {
	textstyleParser := NewTextstyleParser("\\`escaped text\\`")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "`escaped text`")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{})
}

func TestEscapeTilde(t *testing.T) {
	textstyleParser := NewTextstyleParser("\\~escaped text\\~")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "~escaped text~")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{})
}

func TestEscapeNew(t *testing.T) {
	textstyleParser := NewTextstyleParser("Test \\** \\* \\~ Escape")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "Test ** * ~ Escape")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{})
}

func TestBoldItalic(t *testing.T) {
	textstyleParser := NewTextstyleParser("***Bold Italic Text***")
	message, signalCliFormatStrings := textstyleParser.Parse()
	expectMessageEqual(t, message, "Bold Italic Text")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:16:BOLD", "0:16:ITALIC"})
}

func TestStyleChangeWithoutSpace(t *testing.T) {
	message, signalCliFormatStrings := NewTextstyleParser("**bold***italic*").Parse()
	expectMessageEqual(t, message, "bolditalic")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:4:BOLD", "4:6:ITALIC"})

	message, signalCliFormatStrings = NewTextstyleParser("*italic***bold**").Parse()
	expectMessageEqual(t, message, "italicbold")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:6:ITALIC", "6:4:BOLD"})

	message, signalCliFormatStrings = NewTextstyleParser("~strike~**bold**").Parse()
	expectMessageEqual(t, message, "strikebold")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:6:STRIKETHROUGH", "6:4:BOLD"})
}

func TestNestedBoldItalic(t *testing.T) {
	message, signalCliFormatStrings := NewTextstyleParser("***both** italic*").Parse()
	expectMessageEqual(t, message, "both italic")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:4:BOLD", "0:11:ITALIC"})

	message, signalCliFormatStrings = NewTextstyleParser("**bold *both***").Parse()
	expectMessageEqual(t, message, "bold both")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"5:4:ITALIC", "0:9:BOLD"})

	message, signalCliFormatStrings = NewTextstyleParser("*italic **both***").Parse()
	expectMessageEqual(t, message, "italic both")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"7:4:BOLD", "0:11:ITALIC"})

	message, signalCliFormatStrings = NewTextstyleParser("*a**b**c*").Parse()
	expectMessageEqual(t, message, "abc")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"1:1:BOLD", "0:3:ITALIC"})
}

func TestUnmatchedMarkersStayLiteral(t *testing.T) {
	for _, text := range []string{"5 * 3 = 15", "wait ~5 min", "a || b", "**unclosed", "it`s", "x ~ y ~ z"} {
		message, signalCliFormatStrings := NewTextstyleParser(text).Parse()
		expectMessageEqual(t, message, text)
		expectFormatStringsEqual(t, signalCliFormatStrings, []string{})
	}
}

func TestDoubleTildeStrikethrough(t *testing.T) {
	message, signalCliFormatStrings := NewTextstyleParser("~~strike~~").Parse()
	expectMessageEqual(t, message, "strike")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:6:STRIKETHROUGH"})
}

func TestMonospaceContentIsLiteral(t *testing.T) {
	message, signalCliFormatStrings := NewTextstyleParser("`a*b` and `**not bold**`").Parse()
	expectMessageEqual(t, message, "a*b and **not bold**")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:3:MONOSPACE", "8:12:MONOSPACE"})

	message, signalCliFormatStrings = NewTextstyleParser("`a\\`b`").Parse()
	expectMessageEqual(t, message, "a`b")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:3:MONOSPACE"})

	message, signalCliFormatStrings = NewTextstyleParser("**`code`**").Parse()
	expectMessageEqual(t, message, "code")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"0:4:MONOSPACE", "0:4:BOLD"})
}

func TestBackslashes(t *testing.T) {
	message, signalCliFormatStrings := NewTextstyleParser("C:\\Users\\me\\.ssh").Parse()
	expectMessageEqual(t, message, "C:\\Users\\me\\.ssh")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{})

	message, signalCliFormatStrings = NewTextstyleParser("\\\\*a*").Parse()
	expectMessageEqual(t, message, "\\a")
	expectFormatStringsEqual(t, signalCliFormatStrings, []string{"1:1:ITALIC"})
}

func TestFormatTextStyles(t *testing.T) {
	for _, tc := range []struct {
		message string
		styles  []TextStyleRange
		markup  string
	}{
		{"bold and italic", []TextStyleRange{{0, 4, Bold}, {9, 6, Italic}}, "**bold** and *italic*"},
		{"bold italic", []TextStyleRange{{0, 11, Bold}, {5, 6, Italic}}, "**bold *italic***"},
		{"bolditalic", []TextStyleRange{{0, 4, Bold}, {4, 6, Italic}}, "**bold***italic*"},
		{"5 * 3", []TextStyleRange{{0, 5, Bold}}, "**5 \\* 3**"},
		{"a*b", []TextStyleRange{{0, 3, Monospace}}, "`a*b`"},
		{"👋 wave", []TextStyleRange{{3, 4, Spoiler}}, "👋 ||wave||"},
		{"plain", nil, "plain"},
	} {
		if markup := FormatTextStyles(tc.message, tc.styles); markup != tc.markup {
			t.Errorf("FormatTextStyles(%q, %v) = %q, wanted %q", tc.message, tc.styles, markup, tc.markup)
		}
	}
}

var roundTripStyles = []string{Bold, Italic, Strikethrough, Spoiler, Monospace}

// returns false if styles were dropped
func checkRoundTrip(t *testing.T, message string, styles []TextStyleRange) bool {
	t.Helper()
	markup := FormatTextStyles(message, styles)
	text, parsedStyles := parseTextStyles(markup)
	if text != message {
		t.Fatalf("text changed: %q with %v -> %q -> %q", message, styles, markup, text)
	}
	runes := []rune(message)
	want := normalizeMasks(runes, styleMasks(runes, styles))
	got := styleMasks(runes, parsedStyles)
	exact := true
	for i, r := range runes {
		if unicode.IsSpace(r) || got[i] == want[i] {
			continue
		}
		if got[i]&^want[i] != 0 {
			t.Fatalf("styles added: %q with %v -> %q -> %v", message, styles, markup, parsedStyles)
		}
		exact = false
	}
	return exact
}

func randomRanges(rng *rand.Rand, message string) []TextStyleRange {
	length := getUtf16StringLength(message)
	var styles []TextStyleRange
	for k := rng.Intn(5); length > 0 && k > 0; k-- {
		start := rng.Intn(length)
		styles = append(styles, TextStyleRange{start, 1 + rng.Intn(length-start), roundTripStyles[rng.Intn(len(roundTripStyles))]})
	}
	return styles
}

func TestFormatRoundTripWords(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	letters := []rune("ab*~|`\\👋")
	for iteration := 0; iteration < 20000; iteration++ {
		var words []string
		for w := 1 + rng.Intn(6); w > 0; w-- {
			var word []rune
			for l := 1 + rng.Intn(3); l > 0; l-- {
				word = append(word, letters[rng.Intn(len(letters))])
			}
			words = append(words, string(word))
		}
		message := strings.Join(words, " ")

		var styles []TextStyleRange
		for k := rng.Intn(5); k > 0; k-- {
			first := rng.Intn(len(words))
			last := first + rng.Intn(len(words)-first)
			start := getUtf16StringLength(strings.Join(words[:first], " "))
			if first > 0 {
				start++
			}
			end := getUtf16StringLength(strings.Join(words[:last+1], " "))
			styles = append(styles, TextStyleRange{start, end - start, roundTripStyles[rng.Intn(len(roundTripStyles))]})
		}

		if !checkRoundTrip(t, message, styles) {
			t.Errorf("styles lost: %q with %v -> %q", message, styles, FormatTextStyles(message, styles))
		}
	}
}

func TestFormatRoundTripRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	alphabet := []rune("ab  *~|`\\👋\n")
	lossy := 0
	const iterations = 50000
	for iteration := 0; iteration < iterations; iteration++ {
		var message []rune
		for l := rng.Intn(16); l > 0; l-- {
			message = append(message, alphabet[rng.Intn(len(alphabet))])
		}
		if !checkRoundTrip(t, string(message), randomRanges(rng, string(message))) {
			lossy++
		}
	}
	t.Logf("%d of %d random cases needed to drop styles", lossy, iterations)
}

func TestParseFormatParse(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	alphabet := []rune("ab  *~|`\\")
	for iteration := 0; iteration < 50000; iteration++ {
		var input []rune
		for l := rng.Intn(20); l > 0; l-- {
			input = append(input, alphabet[rng.Intn(len(alphabet))])
		}
		message, styles := parseTextStyles(string(input))
		checkRoundTrip(t, message, styles)
	}
}
