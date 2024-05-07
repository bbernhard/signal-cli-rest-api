package utils

import (
	"strconv"
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
	BoldEnd1               = 4
	BoldEnd2               = 5
	MonoSpaceBegin         = 6
	MonoSpaceEnd           = 7
	StrikethroughBegin     = 8
	StrikethroughEnd       = 9
	SpoilerBegin1          = 10
	SpoilerBegin           = 11
	SpoilerEnd1            = 12
	SpoilerEnd2            = 13
)

func getUtf16CharacterCount(s string) int {
	stringLength := len(s)
	if stringLength == 1 {
		return 1
	}
	return stringLength / 2
}

func getAdditionalCharacterCount(characterCount int) int {
	additionalCharacterCount := characterCount - 1
	if additionalCharacterCount > 0 {
		return additionalCharacterCount
	}
	return 0
}

func ParseMarkdownMessage(message string) (string, []string) {
	textFormat := Normal
	textFormatBegin := 0
	textFormatLength := 0
	numOfControlChars := 0
	state := None
	signalCliFormatStrings := []string{}
	fullString := ""
	lastChar := ""
	additionalCharacterCount := 0

	runes := []rune(message) //turn string to slice

	for i, v := range runes { //iterate through rune
		if v == '*' {
			if state == ItalicBegin {
				if lastChar == "*" {
					state = BoldBegin
					textFormat = Bold
					textFormatBegin = i - numOfControlChars + additionalCharacterCount
					textFormatLength = 0
				} else {
					state = ItalicEnd
				}
			} else if state == None {
				state = ItalicBegin
				textFormat = Italic
				textFormatBegin = i - numOfControlChars + additionalCharacterCount
				textFormatLength = 0
			} else if state == BoldBegin {
				state = BoldEnd1
			} else if state == BoldEnd1 {
				state = BoldEnd2
			}
			numOfControlChars += 1
		} else if v == '|' {
			if state == None {
				state = SpoilerBegin1
			} else if state == SpoilerBegin1 && lastChar == "|" {
				state = SpoilerBegin
				textFormat = Spoiler
				textFormatBegin = i - numOfControlChars + additionalCharacterCount
				textFormatLength = 0
			} else if state == SpoilerBegin {
				state = SpoilerEnd1
			} else if state == SpoilerEnd1 && lastChar == "|" {
				state = SpoilerEnd2
			}
			numOfControlChars += 1
		} else if v == '`' {
			if state == None {
				state = MonoSpaceBegin
				textFormat = Monospace
				textFormatBegin = i - numOfControlChars + additionalCharacterCount
				textFormatLength = 0
			} else if state == MonoSpaceBegin {
				state = MonoSpaceEnd
			}
			numOfControlChars += 1
		} else if v == '~' {
			if state == None {
				state = StrikethroughBegin
				textFormat = Strikethrough
				textFormatBegin = i - numOfControlChars + additionalCharacterCount
				textFormatLength = 0
			} else if state == StrikethroughBegin {
				state = StrikethroughEnd
			}
			numOfControlChars += 1
		} else {
			textFormatLength += 1
			fullString += string(v)
			additionalCharacterCount += getAdditionalCharacterCount(getUtf16CharacterCount(string(v)))
		}
		lastChar = string(v)

		if state == ItalicEnd || state == BoldEnd2 || state == MonoSpaceEnd || state == StrikethroughEnd || state == SpoilerEnd2 {
			signalCliFormatStrings = append(signalCliFormatStrings, strconv.Itoa(textFormatBegin)+":"+strconv.Itoa(textFormatLength+additionalCharacterCount)+":"+textFormat)
			state = None
			textFormatBegin = 0
			textFormatLength = 0
			textFormat = Normal
		}
	}

	return fullString, signalCliFormatStrings
}
