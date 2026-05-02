package main

import (
	"fmt"
)

const (
	DefaultBufferSize = 1024 * 8 // 8KB буфер
)

type SimpleArchiver struct {
	inputPath  string
	outputPath string
	buffer     []byte
}

func NewArchiver(inputPath string) *SimpleArchiver {
	return &SimpleArchiver{
		inputPath:  inputPath,
		outputPath: "",
		buffer:     make([]byte, DefaultBufferSize),
	}
}

func (sa *SimpleArchiver) compressEmpty(data []byte) []byte {
	if len(data) == 0 {
		return []byte{}
	}

	return data
}

func (sa *SimpleArchiver) countRepeating(data []byte) []byte {
	if len(data) == 0 {
		return []byte{}
	}

	result := make([]byte, 0, len(data)*2)

	current := data[0]
	count := 1

	for i := 1; i < len(data); i++ {
		if data[i] == current {
			count++
		} else {
			result = append(result, byte(count)+'0', current)
			current = data[i]
			count = 1
		}
	}

	result = append(result, byte(count)+'0', current)

	return result
}

func main() {
	fmt.Println("Happy coding!!!")
}
