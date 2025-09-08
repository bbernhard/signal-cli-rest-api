package utils

import (
	"testing"
)

func expectEqual(t *testing.T, in1 bool, in2 bool) {
	if in1 != in2 {
		t.Errorf("got %t, wanted %t", in1, in2)
	}
}

func TestIsPhoneNumber(t *testing.T) {
	res := IsPhoneNumber("+12345678")	
	expectEqual(t, res, true)
}


func TestIsPhoneNumberWithSpaces(t *testing.T) {
	res := IsPhoneNumber("+ 12345678")	
	expectEqual(t, res, true)
}

func TestIsPhoneNumberWithSpaces1(t *testing.T) {
	res := IsPhoneNumber("+ 1234 5678")	
	expectEqual(t, res, true)
}

func TestIsPhoneNumberWithInvalidCharacters(t *testing.T) {
	res := IsPhoneNumber("+123456x")	
	expectEqual(t, res, false)
}

func TestIsPhoneNumberWithMissingPrefix(t *testing.T) {
	res := IsPhoneNumber("123456x")	
	expectEqual(t, res, false)
}

func TestIsPhoneNumberWithInvalidCharactersAndSpaces(t *testing.T) {
	res := IsPhoneNumber("+12345 6x")	
	expectEqual(t, res, false)
}
