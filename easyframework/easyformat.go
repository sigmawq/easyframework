package easyframework

import (
	"log"
	"reflect"
	"strconv"
	"unsafe"
)

type EasyFormatToken int8

const (
	EASY_FORMAT_TOKEN_INVALID  = 0
	EASY_FORMAT_TOKEN_FIELD_ID = 1
	EASY_FORMAT_TOKEN_END      = 2
)

func Pack(target any) []byte {
	buffer := Buffer{}
	_Pack(&buffer, reflect.TypeOf(target), reflect.ValueOf(target), -1)

	return buffer.Buffer[:buffer.Index]
}

type StructFieldData struct {
	FieldID int
}

var preprocessedStructs map[reflect.Type]map[int]StructFieldData

func PreprocessStruct(theStruct reflect.Type) {
	structMapping := make(map[int]StructFieldData)
	for i := 0; i < theStruct.NumField(); i += 1 {
		field := theStruct.Field(i)
		_id := field.Tag.Get("id")
		if _id == "" {
			continue
		}
		id, _ := strconv.ParseInt(_id, 10, 64)
		if id <= 0 {
			log.Printf("Struct %v: field %v has an ID that is invalid.", theStruct.Name(), id)
			panic("Failed to preprocess struct!")
		}

		_, idAlreadyInUse := structMapping[int(id)]
		if idAlreadyInUse {
			log.Printf("Struct %v: field %v has an ID that is invalid.", theStruct.Name(), id)
			panic("Failed to preprocess struct!")
		}

		isTypeValid := false
		switch field.Type.Kind() {
		case reflect.Bool:
			fallthrough
		case reflect.Int:
			fallthrough
		case reflect.Int8:
			fallthrough
		case reflect.Int16:
			fallthrough
		case reflect.Int32:
			fallthrough
		case reflect.Int64:
			fallthrough
		case reflect.Uint:
			fallthrough
		case reflect.Uint8:
			fallthrough
		case reflect.Uint16:
			fallthrough
		case reflect.Uint32:
			fallthrough
		case reflect.Uint64:
			fallthrough
		case reflect.Float32:
			fallthrough
		case reflect.Float64:
			fallthrough
		case reflect.Complex64:
			fallthrough
		case reflect.Complex128:
			fallthrough
		case reflect.Array:
			fallthrough
		case reflect.Slice:
			fallthrough
		case reflect.String:
			fallthrough
		case reflect.Struct:
			isTypeValid = true
		}

		if !isTypeValid {
			log.Printf("Struct %v: field %v type %v not supported!", theStruct.Name(), field.Name, field.Type.Kind())
			panic("Failed to preprocess struct!")
		}

		structMapping[int(id)] = StructFieldData{
			FieldID: i,
		}
	}

}

func _Pack(buffer *Buffer, targetType reflect.Type, targetValue reflect.Value, fieldID int) {
	if fieldID > 0 {
		CopyToBuffer(buffer, fieldID)
	}
	switch targetType.Kind() {
	case reflect.Bool:
		fallthrough
	case reflect.Int:
		fallthrough
	case reflect.Int8:
		fallthrough
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		fallthrough
	case reflect.Uint:
		fallthrough
	case reflect.Uint8:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint32:
		fallthrough
	case reflect.Uint64:
		fallthrough
	case reflect.Float32:
		fallthrough
	case reflect.Float64:
		fallthrough
	case reflect.Complex64:
		fallthrough
	case reflect.Complex128:
		source := targetValue.Pointer()
		sourceSize := targetType.Size()
		CopyToBufferRaw(buffer, unsafe.Pointer(source), int(sourceSize))
	case reflect.Array:
		panic("not yet implemented")
	case reflect.Slice:
		panic("not yet implemented")
	case reflect.String:
		panic("not yet implemented")
	case reflect.Struct:
		structData, hasStructData := preprocessedStructs[targetType]
		if !hasStructData {
			PreprocessStruct(targetType)
			structData = preprocessedStructs[targetType]
		}

		for fieldID, fieldData := range structData {
			fieldType := targetType.Field(fieldData.FieldID)
			fieldValue := targetValue.Field(fieldData.FieldID)
			_Pack(buffer, fieldType, fieldValue, fieldID)
		}
	default:
		log.Println(targetType.Kind().String())
		panic("Type is unsupported")
	}
}

func Unpack(data []byte) {

}
