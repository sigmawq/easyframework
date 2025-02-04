package easyframework

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"unsafe"
)

type TokenType int8

const (
	FORMAT_TOKEN_INVALID     TokenType = 0
	FORMAT_TOKEN_FIELD_ID    TokenType = 1
	FORMAT_TOKEN_END         TokenType = 2
	FORMAT_TOKEN_ARRAY_INDEX TokenType = 3
	FORMAT_TOKEN_ARRAY_SIZE  TokenType = 4
)

type FieldID int16
type ArrayIndex uint32

func Pack[T any](target *T) ([]byte, error) {
	var buffer Buffer

	err := _Pack(&buffer, reflect.TypeOf(target).Elem(), reflect.ValueOf(target).Elem(), -1)

	return buffer.Buffer[:buffer.Index], err
}

type StructFieldData struct {
	FieldIndex int
	IsRequired bool
}

var preprocessedStructs map[reflect.Type]map[int]StructFieldData

func PreprocessStruct(theStruct reflect.Type) {
	if preprocessedStructs == nil {
		preprocessedStructs = make(map[reflect.Type]map[int]StructFieldData, 0)
	}

	_, alreadyDone := preprocessedStructs[theStruct]
	if alreadyDone {
		return
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

func IsSimpleType(_type reflect.Type) bool {
	simple := false
	switch _type.Kind() {
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
	case reflect.Complex128:
		simple = true
	}

	return simple
}

func _Pack(buffer *Buffer, targetType reflect.Type, targetValue reflect.Value, fieldID int) error {
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
		CopyToBuffer(buffer, float32(targetValue.Float()))
	case reflect.Float64:
		CopyToBuffer(buffer, float64(targetValue.Float()))
	case reflect.Complex64:
		CopyToBuffer(buffer, complex64(targetValue.Complex()))
	case reflect.Complex128:
		CopyToBuffer(buffer, complex128(targetValue.Complex()))
	case reflect.Array:
		if IsSimpleType(targetType.Elem()) {
			pointer := unsafe.Pointer(targetValue.Addr().Pointer())
			elementSize := targetType.Elem().Size()
			CopyToBufferRaw(buffer, pointer, int(elementSize)*targetValue.Len())
		} else {
			for i := 0; i < targetValue.Len(); i++ {
				if targetValue.Index(i).IsZero() {
					continue
				}

				CopyToBuffer(buffer, FORMAT_TOKEN_ARRAY_INDEX)
				CopyToBuffer(buffer, ArrayIndex(i))
				err := _Pack(buffer, targetType.Elem(), targetValue.Index(i), -1)
				if err != nil {
					return err
				}
			}

			CopyToBuffer(buffer, FORMAT_TOKEN_END)
		}
	case reflect.Slice:
		CopyToBuffer(buffer, FORMAT_TOKEN_ARRAY_SIZE)
		CopyToBuffer(buffer, ArrayIndex(targetValue.Len()))

		if IsSimpleType(targetType.Elem()) {
			pointer := unsafe.Pointer(targetValue.Addr().Pointer())
			elementSize := targetType.Elem().Size()
			CopyToBufferRaw(buffer, pointer, int(elementSize)*targetValue.Len())
		} else {
			for i := 0; i < targetValue.Len(); i++ {
				if targetValue.Index(i).IsZero() {
					continue
				}

				CopyToBuffer(buffer, FORMAT_TOKEN_ARRAY_INDEX)
				CopyToBuffer(buffer, ArrayIndex(i))
				err := _Pack(buffer, targetType.Elem(), targetValue.Index(i), -1)
				if err != nil {
					return err
				}
			}

			CopyToBuffer(buffer, FORMAT_TOKEN_END)
		}
	case reflect.String:
		str := ([]byte)(targetValue.String())
		if uint64(len(str)) > 0xffffffff {
			return errors.New(fmt.Sprintf("%v: String is too big!", targetType.Name()))
		}
		CopyToBuffer(buffer, uint32(len(str)))
		CopyToBufferRaw(buffer, unsafe.Pointer(&str[0]), len(str))
	case reflect.Struct:
		PreprocessStruct(targetType)
		structData, _ := preprocessedStructs[targetType]

		for fieldID, fieldData := range structData {
			fieldType := targetType.Field(fieldData.FieldIndex)
			fieldValue := targetValue.Field(fieldData.FieldIndex)

			if !fieldValue.IsZero() {
				_Pack(buffer, fieldType.Type, fieldValue, fieldID)
			}
		}

		CopyToBuffer(buffer, FORMAT_TOKEN_END) // TODO: Don't write this token if that's our root struct (efficiency lol)
	default:
		log.Printf("Type: %v/%v", targetType.Name(), targetType.Kind())
		panic("Impossible: type is unsupported")
	}

	return nil
}

func Unpack[T any](data []byte, target *T) error {
	buffer := Buffer{
		Buffer: data,
	}
	return _Unpack(&buffer, reflect.TypeOf(target).Elem(), reflect.ValueOf(target).Elem())
}

type UnpackError struct {
	Position uint64
	Message  string
}

func (unpackError *UnpackError) Error() string {
	return fmt.Sprintf("Failed to unpack: %v (Position: %v)", unpackError.Message, unpackError.Position)
}

func _Unpack(buffer *Buffer, targetType reflect.Type, targetValue reflect.Value) error {
	switch targetType.Kind() {
	case reflect.Bool:
		var value bool
		if !CopyFromBuffer(buffer, &value) {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "Expected bool, got EOF",
			}
		}

		targetValue.SetBool(value)
	case reflect.Int8:
		var value int8
		if !CopyFromBuffer(buffer, &value) {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "Expected int8, got EOF",
			}
		}

		targetValue.SetInt(int64(value))
	case reflect.Int16:
		var value int16
		if !CopyFromBuffer(buffer, &value) {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "Expected int16, got EOF",
			}
		}

		targetValue.SetInt(int64(value))
	case reflect.Int32:
		var value int32
		if !CopyFromBuffer(buffer, &value) {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "Expected int32, got EOF",
			}
		}

		targetValue.SetInt(int64(value))
	case reflect.Int64:
		var value int64
		if !CopyFromBuffer(buffer, &value) {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "Expected int64, got EOF",
			}
		}

		targetValue.SetInt(int64(value))
	case reflect.Uint8:
		var value uint8
		if !CopyFromBuffer(buffer, &value) {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "Expected uint8, got EOF",
			}
		}

		targetValue.SetInt(int64(value))
	case reflect.Uint16:
		var value uint16
		if !CopyFromBuffer(buffer, &value) {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "Expected uint16, got EOF",
			}
		}

		targetValue.SetInt(int64(value))
	case reflect.Uint32:
		var value uint32
		if !CopyFromBuffer(buffer, &value) {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "Expected uint32, got EOF",
			}
		}

		targetValue.SetInt(int64(value))
	case reflect.Uint64:
		var value uint64
		if !CopyFromBuffer(buffer, &value) {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "Expected uint64, got EOF",
			}
		}

		targetValue.SetInt(int64(value))
	case reflect.Float32:
		var value float32
		if !CopyFromBuffer(buffer, &value) {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "Expected float32, got EOF",
			}
		}

		targetValue.SetFloat(float64(value))
	case reflect.Float64:
		var value float64
		if !CopyFromBuffer(buffer, &value) {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "Expected float64, got EOF",
			}
		}

		targetValue.SetFloat(float64(value))
	case reflect.Complex64:
		var value complex64
		if !CopyFromBuffer(buffer, &value) {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "Expected complex64, got EOF",
			}
		}

		targetValue.SetComplex(complex128(value))
	case reflect.Complex128:
		var value complex128
		if !CopyFromBuffer(buffer, &value) {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "Expected complex128, got EOF",
			}
		}

		targetValue.SetComplex(complex128(value))
	case reflect.Array:
		if IsSimpleType(targetType.Elem()) {
			pointer := unsafe.Pointer(targetValue.Addr().Pointer())
			elementSize := targetType.Elem().Size()
			CopyFromBufferRaw(buffer, pointer, int(elementSize)*targetValue.Len())
		} else {
			for {
				var token TokenType
				CopyFromBuffer(buffer, &token)
				if token == FORMAT_TOKEN_END {
					break
				} else if token != FORMAT_TOKEN_ARRAY_INDEX {
					return &UnpackError{
						Position: uint64(buffer.Index),
						Message:  fmt.Sprintf("Expected FORMAT_TOKEN_ARRAY_INDEX got %v", token),
					}
				}

				var arrayIndex ArrayIndex
				CopyFromBuffer(buffer, &arrayIndex)

				if arrayIndex < 0 || int(arrayIndex) >= targetValue.Len() {
					return &UnpackError{
						Position: uint64(buffer.Index),
						Message:  fmt.Sprintf("Array length %v is outside of array bounds (%v)", arrayIndex, targetValue.Len()),
					}
				}

				err := _Unpack(buffer, targetType.Elem(), targetValue.Index(int(arrayIndex)))
				if err != nil {
					return err
				}
			}
		}
	case reflect.Slice:
		var token TokenType
		CopyFromBuffer(buffer, &token)
		if token != FORMAT_TOKEN_ARRAY_SIZE {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  fmt.Sprintf("Expected FORMAT_TOKEN_ARRAY_SIZE got %v", token),
			}
		}
		var sliceSize ArrayIndex
		CopyFromBuffer(buffer, &sliceSize)
		newSlice := reflect.MakeSlice(targetType, int(sliceSize), int(sliceSize))
		targetValue.Set(newSlice)

		if IsSimpleType(targetType.Elem()) {
			pointer := unsafe.Pointer(targetValue.Addr().Pointer())
			elementSize := targetType.Elem().Size()
			CopyFromBufferRaw(buffer, pointer, int(elementSize)*targetValue.Len())
		} else {
			for {
				var token TokenType
				CopyFromBuffer(buffer, &token)
				if token == FORMAT_TOKEN_END {
					break
				} else if token != FORMAT_TOKEN_ARRAY_INDEX {
					return &UnpackError{
						Position: uint64(buffer.Index),
						Message:  fmt.Sprintf("Expected FORMAT_TOKEN_ARRAY_INDEX got %v", token),
					}
				}

				var arrayIndex ArrayIndex
				CopyFromBuffer(buffer, &arrayIndex)

				if arrayIndex < 0 || int(arrayIndex) >= int(sliceSize) {
					return &UnpackError{
						Position: uint64(buffer.Index),
						Message:  fmt.Sprintf("Array length %v is outside of slice bounds (%v)", arrayIndex, sliceSize),
					}
				}

				err := _Unpack(buffer, targetType.Elem(), targetValue.Index(int(arrayIndex)))
				if err != nil {
					return err
				}
			}
		}
	case reflect.String:
		var stringLength uint32
		if !CopyFromBuffer(buffer, &stringLength) {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "Expected string size (4 bytes), got EOF",
			}
		}
		if stringLength == 0 {
			return &UnpackError{
				Position: uint64(buffer.Index),
				Message:  "String length is zero",
			}
		}

		stringBuffer := make([]byte, stringLength)
		CopyFromBufferRaw(buffer, unsafe.Pointer(&stringBuffer[0]), int(stringLength))
		targetValue.SetString(string(stringBuffer))
	case reflect.Struct:
		structData, hasStructData := preprocessedStructs[targetType]
		if !hasStructData {
			PreprocessStruct(targetType)
			structData = preprocessedStructs[targetType]
		}

		for {
			var token TokenType
			CopyFromBuffer(buffer, &token)
			if token == FORMAT_TOKEN_END {
				break
			} else if token != FORMAT_TOKEN_FIELD_ID {
				return &UnpackError{
					Position: uint64(buffer.Index),
					Message:  fmt.Sprintf("Expected field id token, got another token: %v", token),
				}
			}
			var fieldID uint16
			CopyFromBuffer(buffer, &fieldID)

			if fieldID == 0 {
				return &UnpackError{
					Position: uint64(buffer.Index),
					Message:  fmt.Sprint("Field ID is zero, but it should be greater than zero"),
				}
			}

			fieldData, ok := structData[int(fieldID)]
			if !ok { // We are skipping unknown fields, that's an error in structure mapping, but we don't want to fail just of a single such error
				return &UnpackError{
					Position: uint64(buffer.Index),
					Message:  fmt.Sprintf("Unmapped ID: %v", fieldID),
				}
			}

			if fieldData.FieldIndex >= targetValue.NumField() {
				panic("Impossible: fieldIndex is not mapping a valid struct field")
			}

			fieldType := targetType.Field(fieldData.FieldIndex)
			fieldValue := targetValue.Field(fieldData.FieldIndex)

			err := _Unpack(buffer, fieldType.Type, fieldValue)
			if err != nil {
				return err
			}
		}
	default:
		log.Println(targetType.Kind().String())
		panic("Type is unsupported")
	}

	return nil
}
