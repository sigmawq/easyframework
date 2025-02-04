package easyframework

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"unsafe"
)

func GenerateSixteenDigitCode() string {
	low := 65
	high := 90

	var result [16]byte
	for i := 0; i < 16; i++ {
		char := low + rand.Intn(high-low)
		result[i] = byte(char)
	}

	return string(result[:])
}

func Memcopy(dest, src unsafe.Pointer, len int) unsafe.Pointer {
	cnt := len >> 3
	var i int = 0
	for i = 0; i < cnt; i++ {
		var pdest *uint64 = (*uint64)(unsafe.Pointer(uintptr(dest) + uintptr(8*i)))
		var psrc *uint64 = (*uint64)(unsafe.Pointer(uintptr(src) + uintptr(8*i)))
		*pdest = *psrc
	}
	left := len & 7
	for i = 0; i < left; i++ {
		var pdest *uint8 = (*uint8)(unsafe.Pointer(uintptr(dest) + uintptr(8*cnt+i)))
		var psrc *uint8 = (*uint8)(unsafe.Pointer(uintptr(src) + uintptr(8*cnt+i)))

		*pdest = *psrc
	}
	return dest
}

type Buffer struct {
	Buffer []byte
	Index  int
}

func BufferGrowAtLeast(buffer *Buffer, minimumSize int) {
	oldBuffer := buffer.Buffer
	oldSize := len(buffer.Buffer)
	newSize := oldSize * 2
	if newSize == 0 {
		newSize = 16
	}
	if newSize < minimumSize {
		newSize = minimumSize
	}
	newBuffer := make([]byte, newSize)

	buffer.Buffer = newBuffer
	if len(oldBuffer) > 0 {
		Memcopy(unsafe.Pointer(&newBuffer[0]), unsafe.Pointer(&oldBuffer[0]), oldSize)
	}
}

func CopyToBufferRaw(buffer *Buffer, pointer unsafe.Pointer, size int) {
	if buffer.Index+size > len(buffer.Buffer) {
		BufferGrowAtLeast(buffer, len(buffer.Buffer)+size)
	}

	Memcopy(unsafe.Pointer(&buffer.Buffer[buffer.Index]), pointer, size)
	buffer.Index += size
}

func CopyToBuffer[T any](buffer *Buffer, thing T) {
	size := int(unsafe.Sizeof(thing))
	if buffer.Index+size > len(buffer.Buffer) {
		BufferGrowAtLeast(buffer, len(buffer.Buffer)+size)
	}

	Memcopy(unsafe.Pointer(&buffer.Buffer[buffer.Index]), unsafe.Pointer(&thing), size)
	buffer.Index += size
}

func CopyFromBufferRaw(buffer *Buffer, pointer unsafe.Pointer, size int) bool {
	if buffer.Index+size > len(buffer.Buffer) {
		return false
	}

	Memcopy(pointer, unsafe.Pointer(&buffer.Buffer[buffer.Index]), size)
	buffer.Index += size
	return true
}

func CopyFromBuffer[T any](buffer *Buffer, thing *T) bool {
	size := int(unsafe.Sizeof(*thing))
	if buffer.Index+size > len(buffer.Buffer) {
		return false
	}

	Memcopy(unsafe.Pointer(thing), unsafe.Pointer(&buffer.Buffer[buffer.Index]), size)
	buffer.Index += size
	return true
}

func GetTrace(callerHeight int) string {
	_, file, no, _ := runtime.Caller(callerHeight)

	start := 0
	for i, char := range file {
		if char == '\\' || char == '/' {
			start = i + 1
		}
	}

	if start < len(file) {
		file = file[start:]
	}

	return fmt.Sprintf("%v:%v", file, no)
}

func GetCallerFunctionName(callerHeight int) string {
	pc, _, _, _ := runtime.Caller(callerHeight)
	return fmt.Sprintf("%v", runtime.FuncForPC(pc).Name())
}

func RJson[T any](w http.ResponseWriter, status int, value T) {
	data, err := json.Marshal(value)
	if err != nil {
		log.Printf("Error while trying to marshal json to send it as response: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, string(data))
}

func String200(w http.ResponseWriter, str string) {
	w.WriteHeader(200)
	fmt.Fprint(w, str)
}

func Search[T any](array []T, eq func(v T) bool) (T, bool) {
	for _, v := range array {
		if eq(v) {
			return v, true
		}
	}

	// NOTE: you can't to T{} for retarded reason
	var v T
	return v, false
}

func SearchI[T any](array []T, eq func(v T) bool) (int, bool) {
	for i, v := range array {
		if eq(v) {
			return i, true
		}
	}

	return 0, false
}

func SearchPtr[T any](array []T, eq func(v *T) bool) (*T, bool) {
	for i, _ := range array {
		if eq(&array[i]) {
			return &array[i], true
		}
	}

	return nil, false
}

func Remove[T any](array []T, i int) []T {
	array[i] = array[len(array)-1]
	return array[:len(array)-1]
}

func CreateDirectoryIfDoesntExist(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.Mkdir(path, 0777)
		if err != nil {
			panic(err)
		}
	}
}
