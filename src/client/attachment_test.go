package client

import (
	"strings"
	"testing"
)

func Test_Attachment_ExtractMetadata_ShouldPrepareDataFor_toDataForSignal(t *testing.T) {
	testCases := []struct {
		nameTest             string
		inputData            string
		resultIsWithMetaData bool
		base64Expected       string
		fileNameExpected     string
		mimeInfoExpected     string
		toDataForSignal      string
	}{
		{
			"BC base64 - compatibility", "MTIzNDU=", false, "MTIzNDU=", "", "", "MTIzNDU=",
		},
		{
			"+base64 -data -filename", "base64,MTIzNDU=", false, "base64,MTIzNDU=", "", "", "base64,MTIzNDU=",
		},
		{
			"+base64 +data -filename", "data:someData;base64,MTIzNDU=", true, "MTIzNDU=", "", "someData", "data:someData;base64,MTIzNDU=",
		},
		{
			"+base64 -data +filename", "filename=file.name;base64,MTIzNDU=", false, "filename=file.name;base64,MTIzNDU=", "", "", "filename=file.name;base64,MTIzNDU=",
		},
		{
			"+base64 +data +filename", "data:someData;filename=file.name;base64,MTIzNDU=", true, "MTIzNDU=", "file.name", "someData", "data:someData;filename=file.name;base64,MTIzNDU=",
		},
		{
			"-base64 -data -filename", "INVALIDMTIzNDU=", false, "INVALIDMTIzNDU=", "", "", "INVALIDMTIzNDU=",
		},
		{
			"-base64 +data -filename", "data:someData;INVALIDMTIzNDU=", false, "data:someData;INVALIDMTIzNDU=", "", "", "data:someData;INVALIDMTIzNDU=",
		},
		{
			"-base64 -data +filename", "filename=file.name;INVALIDMTIzNDU=", false, "filename=file.name;INVALIDMTIzNDU=", "", "", "filename=file.name;INVALIDMTIzNDU=",
		},
		{
			"-base64 +data +filename", "data:someData;filename=file.name;INVALIDMTIzNDU=", false, "data:someData;filename=file.name;INVALIDMTIzNDU=", "", "", "data:someData;filename=file.name;INVALIDMTIzNDU=",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.nameTest, func(t *testing.T) {
			attachmentEntry := NewAttachmentEntry(tt.inputData)

			if attachmentEntry.isWithMetaData() != tt.resultIsWithMetaData {
				t.Errorf("isWithMetaData() got \"%v\", want \"%v\"", attachmentEntry.isWithMetaData(), tt.resultIsWithMetaData)
			}

			if strings.Compare(attachmentEntry.Base64, tt.base64Expected) != 0 {
				t.Errorf("Base64 got \"%v\", want \"%v\"", attachmentEntry.Base64, tt.base64Expected)
			}

			if strings.Compare(attachmentEntry.FileName, tt.fileNameExpected) != 0 {
				t.Errorf("FileName got \"%v\", want \"%v\"", attachmentEntry.FileName, tt.fileNameExpected)
			}

			if strings.Compare(attachmentEntry.MimeInfo, tt.mimeInfoExpected) != 0 {
				t.Errorf("MimeInfo got \"%v\", want \"%v\"", attachmentEntry.MimeInfo, tt.mimeInfoExpected)
			}

			if strings.Compare(attachmentEntry.toDataForSignal(), tt.toDataForSignal) != 0 {
				t.Errorf("toDataForSignal() got \"%v\", want \"%v\"", attachmentEntry.toDataForSignal(), tt.toDataForSignal)
			}
		})
	}
}
