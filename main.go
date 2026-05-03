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

func (sa *SimpleArchiver) createControlByte(count int, isCompressed bool) byte {
	if count > 127 {
		count = 127
	}

	if isCompressed {
		return byte(128 + count)
	}

	return byte(count)
}

func (sa *SimpleArchiver) scanLookaheadGroups(data []byte) []byte {
	result := []byte{}
	i := 0

	for i < len(data) {
		runLen := 1
		for i+runLen < len(data) && data[i+runLen] == data[i] {
			runLen++
		}

		if runLen >= 4 {
			compressed := sa.countRepeating(data[i : i+runLen])
			ctrl := sa.createControlByte(runLen, true)
			result = append(result, ctrl)
			result = append(result, compressed...)
			i += runLen
		} else {
			groupStart := i

			for i < len(data) {
				nextRun := 1
				for i+nextRun < len(data) && data[i+nextRun] == data[i] {
					nextRun++
				}

				if nextRun >= 4 {
					break
				}

				i += nextRun
			}

			groupLen := i - groupStart
			ctrl := sa.createControlByte(groupLen, false)
			result = append(result, ctrl)
			result = append(result, data[groupStart:i]...)
		}
	}

	return result
}

func main() {
	fmt.Println("Happy coding!!!")
}
