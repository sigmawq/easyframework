package easyframework

import "C"
import (
	"math/rand"
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

func Memcopy(dst, src unsafe.Pointer, size int) {
	C.memcpy(dst, src, size)
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

	Memcopy(unsafe.Pointer(&newBuffer[0]), unsafe.Pointer(&oldBuffer[0]), oldSize)
	buffer.Buffer = newBuffer
}

func CopyToBufferRaw(buffer *Buffer, pointer unsafe.Pointer, size int) {
	if buffer.Index+size > len(buffer.Buffer) {
		BufferGrowAtLeast(buffer, size)
	}

	Memcopy(unsafe.Pointer(&buffer.Buffer[buffer.Index]), pointer, size)
	buffer.Index += size
}

func CopyToBuffer[T any](buffer *Buffer, thing T) {
	size := int(unsafe.Sizeof(thing))
	if buffer.Index+size > len(buffer.Buffer) {
		BufferGrowAtLeast(buffer, size)
	}

	Memcopy(unsafe.Pointer(&buffer.Buffer[buffer.Index]), unsafe.Pointer(&thing), size)
	buffer.Index += size
}
