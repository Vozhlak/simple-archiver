package main

import "fmt"

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

func main() {
	fmt.Println("Happy coding!!!")
}
