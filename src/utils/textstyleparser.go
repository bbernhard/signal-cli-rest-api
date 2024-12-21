package utils

import (
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	Normal        string = "NORMAL"
	Bold                 = "BOLD"
	Italic               = "ITALIC"
	Monospace            = "MONOSPACE"
	Strikethrough        = "STRIKETHROUGH"
	Spoiler              = "SPOILER"
)

const (
	None               int = 0
	ItalicBegin            = 1
	ItalicEnd              = 2
	BoldBegin              = 3
	MonoSpaceBegin         = 6
	StrikethroughBegin     = 8
	SpoilerBegin           = 9
)

func getUtf16StringLength(s string) int {
	runes := []rune(s) //turn string to slice

	length := 0
	for _, r := range runes {
		length += utf16.RuneLen(r)
	}
	return length
}

type TokenState struct {
	BeginPos int
	Token    int
}

type Stack []TokenState

func (s *Stack) Push(v TokenState) {
	*s = append(*s, v)
}

func (s *Stack) Pop() TokenState {
	ret := (*s)[len(*s)-1]
	*s = (*s)[0 : len(*s)-1]

	return ret
}

func (s *Stack) Peek() TokenState {
	ret := (*s)[len(*s)-1]
	return ret
}

func (s *Stack) Empty() bool {
	if len(*s) == 0 {
		return true
	}
	return false
}

const eof = -1

type TextstyleParser struct {
	input                  string
	pos                    int
	width                  int
	tokens                 Stack
	fullString             string
	signalCliFormatStrings []string
	//numOfControlTokens int
}

func NewTextstyleParser(input string) *TextstyleParser {
	return &TextstyleParser{
		input:                  input,
		pos:                    0,
		width:                  0,
		tokens:                 make(Stack, 0),
		fullString:             "",
		signalCliFormatStrings: []string{},
	}
}

func (l *TextstyleParser) next() (rune rune) {
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	//r := []rune(l.input[l.pos:])[0]
	//l.width = utf16.RuneLen(r)
	//l.pos += l.width
	rune, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	return rune
}

// backup steps back one rune.
// Can be called only once per call of next.
func (l *TextstyleParser) backup() {
	l.pos -= l.width
}

// peek returns but does not consume
// the next rune in the input.
func (l *TextstyleParser) peek() rune {
	rune := l.next()
	l.backup()
	return rune
}

func (l *TextstyleParser) handleToken(tokenType int, signalCliStylingType string) {
	if l.tokens.Empty() {
		l.tokens.Push(TokenState{BeginPos: getUtf16StringLength(l.fullString), Token: tokenType})
	} else {
		if l.tokens.Peek().Token == tokenType {
			tokenBeginState := l.tokens.Pop()
			l.signalCliFormatStrings = append(l.signalCliFormatStrings, strconv.Itoa(tokenBeginState.BeginPos)+":"+strconv.Itoa(getUtf16StringLength(l.fullString)-tokenBeginState.BeginPos)+":"+signalCliStylingType)
		} else {
			l.tokens.Push(TokenState{BeginPos: getUtf16StringLength(l.fullString), Token: tokenType})
		}
	}
}

func (l *TextstyleParser) Parse() (string, []string) {
	for {
		c := l.next()
		if c == eof {
			break
		}

		nextRune := l.peek()

		if c == '*' {
			if nextRune == '*' { //Bold
				l.next()
				l.handleToken(BoldBegin, Bold)
			} else { //Italic
				l.handleToken(ItalicBegin, Italic)
			}
		} else if (c == '|') && (nextRune == '|') {
			l.next()
			l.handleToken(SpoilerBegin, Spoiler)
		} else if c == '~' {
			l.handleToken(StrikethroughBegin, Strikethrough)
		} else if c == '`' {
			l.handleToken(MonoSpaceBegin, Monospace)
		} else {
			l.fullString += string(c)
		}
	}

	return l.fullString, l.signalCliFormatStrings
}
