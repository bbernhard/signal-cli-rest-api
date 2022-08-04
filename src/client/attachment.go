package client

import (
	"encoding/base64"
	"errors"
	"os"
	"reflect"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	uuid "github.com/gofrs/uuid"
)

type AttachmentEntry struct {
	MimeInfo         string
	FileName         string
	DirName          string
	Base64           string
	FilePath         string
	attachmentTmpDir string
}

func NewAttachmentEntry(attachmentData string, attachmentTmpDir string) *AttachmentEntry {
	attachmentEntry := AttachmentEntry{
		MimeInfo:         "",
		FileName:         "",
		DirName:          "",
		Base64:           "",
		FilePath:         "",
		attachmentTmpDir: attachmentTmpDir,
	}

	attachmentEntry.extractMetaData(attachmentData)

	return &attachmentEntry
}

func (attachmentEntry *AttachmentEntry) extractMetaData(attachmentData string) {
	base64FlagIndex := strings.LastIndex(attachmentData, "base64,")

	if !strings.Contains(attachmentData, "data:") || base64FlagIndex == -1 {
		attachmentEntry.Base64 = attachmentData
		return
	}

	attachmentEntry.Base64 = attachmentData[base64FlagIndex+len("base64,"):]
	metaDataKeys := map[string]string{
		"data:":     "MimeInfo",
		"filename=": "FileName",
	}

	for _, metaDataLineItem := range strings.Split(attachmentData[:base64FlagIndex-1], ";") {
		for metaDataKey, metaDataFieldName := range metaDataKeys {
			flagIndex := strings.Index(metaDataLineItem, metaDataKey)

			if flagIndex != 0 {
				continue
			}

			attachmentEntry.setFieldValueByName(metaDataFieldName, metaDataLineItem[len(metaDataKey):])
		}
	}
}

func (attachmentEntry *AttachmentEntry) storeBase64AsTemporaryFile() error {
	if strings.Compare(attachmentEntry.Base64, "") == 0 {
		return errors.New("The base64 data does not exist.")
	}

	dec, err := base64.StdEncoding.DecodeString(attachmentEntry.Base64)
	if err != nil {
		return err
	}

	// if no custom filename
	if strings.Compare(attachmentEntry.FileName, "") == 0 {
		mimeType := mimetype.Detect(dec)

		fileNameUuid, err := uuid.NewV4()
		if err != nil {
			return err
		}
		attachmentEntry.FileName = fileNameUuid.String() + mimeType.Extension()
	}

	dirNameUuid, err := uuid.NewV4()
	if err != nil {
		return err
	}

	attachmentEntry.DirName = dirNameUuid.String()
	dirPath := attachmentEntry.attachmentTmpDir + attachmentEntry.DirName
	if err := os.Mkdir(dirPath, os.ModePerm); err != nil {
		return err
	}

	attachmentEntry.FilePath = dirPath + string(os.PathSeparator) + attachmentEntry.FileName

	f, err := os.Create(attachmentEntry.FilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(dec); err != nil {
		attachmentEntry.cleanUp()
		return err
	}
	if err := f.Sync(); err != nil {
		attachmentEntry.cleanUp()
		return err
	}
	f.Close()

	return nil
}

func (attachmentEntry *AttachmentEntry) cleanUp() {
	if strings.Compare(attachmentEntry.FilePath, "") != 0 {
		os.Remove(attachmentEntry.FilePath)
	}

	if strings.Compare(attachmentEntry.DirName, "") != 0 {
		dirPath := attachmentEntry.attachmentTmpDir + attachmentEntry.DirName
		os.Remove(dirPath)
	}
}

func (attachmentEntry *AttachmentEntry) setFieldValueByName(fieldName string, fieldValue string) {
	reflectPointer := reflect.ValueOf(attachmentEntry)
	reflectStructure := reflectPointer.Elem()

	if reflectStructure.Kind() != reflect.Struct {
		return
	}

	field := reflectStructure.FieldByName(fieldName)
	if !field.IsValid() {
		return
	}

	if !field.CanSet() || field.Kind() != reflect.String {
		return
	}

	field.SetString(fieldValue)
}

func (attachmentEntry *AttachmentEntry) isWithMetaData() bool {
	return len(attachmentEntry.MimeInfo) > 0 && len(attachmentEntry.Base64) > 0
}

func (attachmentEntry *AttachmentEntry) toDataForSignal() string {
	if len(attachmentEntry.FilePath) > 0 {
		return attachmentEntry.FilePath
	}

	if !attachmentEntry.isWithMetaData() {
		return attachmentEntry.Base64
	}

	result := "data:" + attachmentEntry.MimeInfo

	if len(attachmentEntry.FileName) > 0 {
		result = result + ";filename=" + attachmentEntry.FileName
	}

	return result + ";base64," + attachmentEntry.Base64
}
