package utils

import (
	"strconv"
)

const (
	Normal string = "NORMAL"
	Bold          = "BOLD"
	Italic        = "ITALIC"
	Monospace     = "MONOSPACE"
	Strikethrough = "STRIKETHROUGH"
)

const (
	None        int    = 0
	ItalicBegin        = 1
	ItalicEnd          = 2
	BoldBegin          = 3
	BoldEnd1           = 4
	BoldEnd2           = 5
	MonoSpaceBegin     = 6
	MonoSpaceEnd       = 7
	StrikethroughBegin = 8
	StrikethroughEnd   = 9
)

func ParseMarkdownMessage(message string) (string, []string) {
	textFormat := Normal
	textFormatBegin := 0
	textFormatLength := 0
	numOfControlChars := 0
	state := None
	signalCliFormatStrings := []string{}
	fullString := ""
	lastChar := ""

	runes := []rune(message) //turn string to slice

	for i, v := range runes { //iterate through rune
		if v == '*' {
			if state == ItalicBegin {
				if lastChar == "*" {
					state = BoldBegin
					textFormat = Bold
					textFormatBegin = i - numOfControlChars
					textFormatLength = 0
				} else {
					state = ItalicEnd
				}
			} else if state == None {
				state = ItalicBegin
				textFormat = Italic
				textFormatBegin = i - numOfControlChars
				textFormatLength = 0
			} else if state == BoldBegin {
				state = BoldEnd1
			} else if state == BoldEnd1 {
				state = BoldEnd2
			}
			numOfControlChars += 1
		} else if v == '`' {
			if state == None {
				state = MonoSpaceBegin
				textFormat = Monospace
				textFormatBegin = i - numOfControlChars
				textFormatLength = 0
			} else if state == MonoSpaceBegin {
				state = MonoSpaceEnd
			}
			numOfControlChars += 1
		} else if v == '~' {
			if state == None {
				state = StrikethroughBegin
				textFormat = Strikethrough
				textFormatBegin = i - numOfControlChars
				textFormatLength = 0
			} else if state == StrikethroughBegin {
				state = StrikethroughEnd
			}
			numOfControlChars += 1
		} else {
			textFormatLength += 1
			fullString += string(v)
		}
		lastChar = string(v)

		if state == ItalicEnd || state == BoldEnd2 || state == MonoSpaceEnd || state == StrikethroughEnd {
			signalCliFormatStrings = append(signalCliFormatStrings, strconv.Itoa(textFormatBegin)+":"+strconv.Itoa(textFormatLength)+":"+textFormat)
			state = None
			textFormatBegin = 0
			textFormatLength = 0
			textFormat = Normal
		}
	}

	return fullString, signalCliFormatStrings
}
