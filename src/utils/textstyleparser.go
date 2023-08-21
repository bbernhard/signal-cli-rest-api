package utils

import (
	"strconv"
)

const (
	Normal string = "NORMAL"
	Bold          = "BOLD"
	Italic        = "ITALIC"
)

const (
	None        int   = 0
	ItalicBegin       = 1
	ItalicEnd         = 2
	BoldBegin         = 3
	BoldEnd1          = 4
	BoldEnd2          = 5
)

func ParseMarkdownMessage(message string) (string, []string) {
	textFormat := Normal
	textFormatBegin := 0
	textFormatLength := 0
	numOfAsterisks := 0
	state := None
	signalCliFormatStrings := []string{}
	fullString := ""

	runes := []rune(message) //turn string to slice

	for i, v := range runes { //iterate through rune
		if v == '*' {
			if state == ItalicBegin {
				if i-1 == textFormatBegin {
					state = BoldBegin
					textFormat = Bold
					textFormatBegin = i - numOfAsterisks
					textFormatLength = 0
				} else {
					state = ItalicEnd
				}
			} else if state == None {
				state = ItalicBegin
				textFormat = Italic
				textFormatBegin = i - numOfAsterisks
				textFormatLength = 0
			} else if state == BoldBegin {
				state = BoldEnd1
			} else if state == BoldEnd1 {
				state = BoldEnd2
			}
			numOfAsterisks += 1
		} else {
			textFormatLength += 1
			fullString += string(v)
		}

		if state == ItalicEnd || state == BoldEnd2 {
			signalCliFormatStrings = append(signalCliFormatStrings, strconv.Itoa(textFormatBegin)+":"+strconv.Itoa(textFormatLength)+":"+textFormat)
			state = None
			textFormatBegin = 0
			textFormatLength = 0
			textFormat = Normal
		}
	}

	return fullString, signalCliFormatStrings
}
