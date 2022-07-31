package client

import (
	"reflect"
	"strings"
)

type AttachmentEntry struct {
	MimeInfo string
	FileName string
	Base64   string
	FilePath string
}

func NewAttachmentEntry(attachmentData string) *AttachmentEntry {
	attachmentEntry := AttachmentEntry{
		MimeInfo: "",
		FileName: "",
		Base64:   "",
		FilePath: "",
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
