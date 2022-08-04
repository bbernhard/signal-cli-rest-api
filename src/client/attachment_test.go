package client

import (
	"flag"
	"os"
	"strings"
	"testing"
)

func Test_Attachment_ExtractMetadata_ShouldPrepareDataFor_toDataForSignal(t *testing.T) {
	testCases := []struct {
		nameTest             string
		inputData            string
		resultIsWithMetaData bool
		base64Expected       string
		base64Valid          bool
		fileNameExpected     string
		mimeInfoExpected     string
		toDataForSignal      string
	}{
		{
			"BC base64 - compatibility", "MTIzNDU=", false, "MTIzNDU=", true, "", "", "MTIzNDU=",
		},
		{
			"+base64 -data -filename", "base64,MTIzNDU=", false, "base64,MTIzNDU=", false, "", "", "base64,MTIzNDU=",
		},
		{
			"+base64 +data -filename", "data:someData;base64,MTIzNDU=", true, "MTIzNDU=", true, "", "someData", "data:someData;base64,MTIzNDU=",
		},
		{
			"+base64 -data +filename", "filename=file.name;base64,MTIzNDU=", false, "filename=file.name;base64,MTIzNDU=", false, "", "", "filename=file.name;base64,MTIzNDU=",
		},
		{
			"+base64 +data +filename", "data:someData;filename=file.name;base64,MTIzNDU=", true, "MTIzNDU=", true, "file.name", "someData", "data:someData;filename=file.name;base64,MTIzNDU=",
		},
		{
			"-base64 -data -filename", "INVALIDMTIzNDU=", false, "INVALIDMTIzNDU=", false, "", "", "INVALIDMTIzNDU=",
		},
		{
			"-base64 +data -filename", "data:someData;INVALIDMTIzNDU=", false, "data:someData;INVALIDMTIzNDU=", false, "", "", "data:someData;INVALIDMTIzNDU=",
		},
		{
			"-base64 -data +filename", "filename=file.name;INVALIDMTIzNDU=", false, "filename=file.name;INVALIDMTIzNDU=", false, "", "", "filename=file.name;INVALIDMTIzNDU=",
		},
		{
			"-base64 +data +filename", "data:someData;filename=file.name;INVALIDMTIzNDU=", false, "data:someData;filename=file.name;INVALIDMTIzNDU=", false, "", "", "data:someData;filename=file.name;INVALIDMTIzNDU=",
		},
	}

	attachmentTmp := flag.String("attachment-tmp-dir", string(os.PathSeparator)+"tmp"+string(os.PathSeparator), "Attachment tmp directory")

	for _, tt := range testCases {
		t.Run(tt.nameTest, func(t *testing.T) {
			attachmentEntry := NewAttachmentEntry(tt.inputData, *attachmentTmp)

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

			err := attachmentEntry.storeBase64AsTemporaryFile()
			if err != nil && tt.base64Valid {
				t.Error("storeBase64AsTemporaryFile error: %w", err)
				return
			}

			info, err := os.Stat(attachmentEntry.FilePath)
			if os.IsNotExist(err) && tt.base64Valid {
				t.Error("file not exists after storeBase64AsTemporaryFile: %w", err)
				return
			}
			if (info == nil || info.IsDir()) && tt.base64Valid {
				t.Error("is not a file by path after storeBase64AsTemporaryFile")
				t.Error(attachmentEntry)
				return
			}

			attachmentEntry.cleanUp()

			info, err = os.Stat(attachmentEntry.FilePath)
			if info != nil && !os.IsNotExist(err) && tt.base64Valid {
				t.Error("no info or file exists after cleanUp")
				return
			}
			info, err = os.Stat(*attachmentTmp + attachmentEntry.DirName)
			if info != nil && !os.IsNotExist(err) && tt.base64Valid {
				t.Error("dir exists after cleanUp")
				return
			}
		})
	}
}
