package client

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ds "github.com/bbernhard/signal-cli-rest-api/datastructs"
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
		{
			"base64 prefix at start +data", "base64,data:someData", false, "data:someData", false, "", "", "data:someData",
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

func Test_Attachment_StoreBase64AsTemporaryFile_RejectsFileNamesWithPaths(t *testing.T) {
	for _, name := range []string{".", "..", "../../outside", "/outside", "dir/file", `dir\file`, "file\x00name"} {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			tmpDir := filepath.Join(parent, "tmp") + string(os.PathSeparator)
			if err := os.Mkdir(tmpDir, 0o700); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(parent, "outside")
			if err := os.WriteFile(outside, []byte("original"), 0o600); err != nil {
				t.Fatal(err)
			}

			attachmentEntry := &AttachmentEntry{Base64: "aGVsbG8=", FileName: name, attachmentTmpDir: tmpDir}
			if err := attachmentEntry.storeBase64AsTemporaryFile(); err == nil {
				t.Errorf("storeBase64AsTemporaryFile accepted filename %q", name)
			}
			attachmentEntry.cleanUp()

			entries, err := os.ReadDir(tmpDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Errorf("attachment tmp dir has entries %v", entries)
			}
			content, err := os.ReadFile(outside)
			if err != nil || string(content) != "original" {
				t.Errorf("file next to the tmp dir got %q, %v; want \"original\"", content, err)
			}
		})
	}
}

func Test_Send_RemovesAttachmentsWhenJsonRpcClientIsMissing(t *testing.T) {
	tmpDir := t.TempDir() + string(os.PathSeparator)
	signalClient := &SignalClient{
		attachmentTmpDir: tmpDir,
		signalCliMode:    JsonRpc,
		jsonRpc2Clients:  map[string]*JsonRpc2Client{},
	}

	_, err := signalClient.send(ds.SignalCliSendRequest{
		Number:            "+4915100000000",
		Recipients:        []string{"+4915100000001"},
		RecipientType:     ds.Number,
		Message:           "hello",
		Base64Attachments: []string{"data:text/plain;filename=hello.txt;base64,aGVsbG8="},
	})
	if err == nil {
		t.Fatal("send succeeded without a JSON-RPC client")
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("attachment tmp dir has entries %v after the failed send", entries)
	}
}
