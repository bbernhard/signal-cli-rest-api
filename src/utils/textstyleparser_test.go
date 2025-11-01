package utils

import (
	"reflect"
	"testing"
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
