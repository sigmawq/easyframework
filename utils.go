package easyframework

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
