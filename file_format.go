package easyframework

import (
	"log"
	"reflect"
	"strconv"
)

type TokenType int8

const (
	FORMAT_TOKEN_INVALID  TokenType = 0
	FORMAT_TOKEN_FIELD_ID TokenType = 1
	FORMAT_TOKEN_END      TokenType = 2
)

type FieldID int16

func Pack[T any](target *T) []byte {
	var buffer Buffer

	valueOf := reflect.ValueOf(*target)
	if valueOf.IsZero() {
		return nil
	}

	_Pack(&buffer, reflect.TypeOf(*target), reflect.ValueOf(*target), -1)

	return buffer.Buffer[:buffer.Index]
}

type StructFieldData struct {
	FieldIndex int
}

var preprocessedStructs map[reflect.Type]map[int]StructFieldData

func PreprocessStruct(theStruct reflect.Type) {
	if preprocessedStructs == nil {
		preprocessedStructs = make(map[reflect.Type]map[int]StructFieldData, 0)
	}

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

		case reflect.Int:
			panic("Raw integer type is not supported, we need to know the exact size of your variable")
		case reflect.Uint:
			panic("Raw integer type is not supported, we need to know the exact size of your variable")
		case reflect.Bool:
			fallthrough
		case reflect.Int8:
			fallthrough
		case reflect.Int16:
			fallthrough
		case reflect.Int32:
			fallthrough
		case reflect.Int64:
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
			FieldIndex: i,
		}
	}

	preprocessedStructs[theStruct] = structMapping
}

func _Pack(buffer *Buffer, targetType reflect.Type, targetValue reflect.Value, fieldID int) {
	if fieldID > 0 {
		CopyToBuffer(buffer, FORMAT_TOKEN_FIELD_ID)
		CopyToBuffer(buffer, FieldID(fieldID))
	}
	switch targetType.Kind() {
	case reflect.Bool:
		CopyToBuffer(buffer, targetValue.Bool())
	case reflect.Int8:
		CopyToBuffer(buffer, int8(targetValue.Int()))
	case reflect.Int16:
		CopyToBuffer(buffer, int16(targetValue.Int()))
	case reflect.Int32:
		CopyToBuffer(buffer, int32(targetValue.Int()))
	case reflect.Int64:
		CopyToBuffer(buffer, int64(targetValue.Int()))
	case reflect.Uint8:
		CopyToBuffer(buffer, uint8(targetValue.Int()))
	case reflect.Uint16:
		CopyToBuffer(buffer, uint16(targetValue.Int()))
	case reflect.Uint32:
		CopyToBuffer(buffer, uint32(targetValue.Int()))
	case reflect.Uint64:
		CopyToBuffer(buffer, uint64(targetValue.Int()))
	case reflect.Float32:
		CopyToBuffer(buffer, float32(targetValue.Int()))
	case reflect.Float64:
		CopyToBuffer(buffer, float64(targetValue.Int()))
	case reflect.Complex64:
		CopyToBuffer(buffer, complex64(targetValue.Complex()))
	case reflect.Complex128:
		CopyToBuffer(buffer, complex128(targetValue.Complex()))
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
			fieldType := targetType.Field(fieldData.FieldIndex)
			fieldValue := targetValue.Field(fieldData.FieldIndex)

			if !fieldValue.IsZero() {
				_Pack(buffer, fieldType.Type, fieldValue, fieldID)
			}
		}

		CopyToBuffer(buffer, FORMAT_TOKEN_END)
	default:
		log.Println(targetType.Kind().String())
		panic("Type is unsupported")
	}
}

func Unpack[T any](data []byte, target *T) error {
	return nil
}
