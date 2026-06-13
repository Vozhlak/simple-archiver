package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

func (sa *SimpleArchiver) collectUncompressedGroup(data []byte, start int) int {
	i := start
	for i < len(data) {
		nextRun := 1
		for i+nextRun < len(data) && data[i+nextRun] == data[i] {
			nextRun++
		}
		if nextRun >= 3 {
			break
		}
		i += nextRun
	}
	return i
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
			groupEnd := sa.collectUncompressedGroup(data, i)
			groupLen := groupEnd - i

			ctrl := sa.createControlByte(groupLen, false)
			result = append(result, ctrl)
			result = append(result, data[i:groupEnd]...)
			i = groupEnd
		}
	}

	return result
}

func (sa *SimpleArchiver) compress(data []byte) []byte {
	if len(sa.compressEmpty(data)) == 0 {
		return []byte{}
	}

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
			result = append(result, compressed[len(compressed)-1])
			i += runLen
		} else {
			groupStart := i
			i++

			groupEnd := sa.collectUncompressedGroup(data, i)
			groupLen := groupEnd - groupStart

			ctrl := sa.createControlByte(groupLen, false)
			result = append(result, ctrl)
			result = append(result, data[groupStart:groupEnd]...)
			i = groupEnd
		}
	}
	return result
}

func (sa *SimpleArchiver) decompress(data []byte) []byte {
	if len(data) == 0 {
		return []byte{}
	}

	result := make([]byte, 0, len(data))
	i := 0

	for i < len(data) {
		control := data[i]
		i++

		isCompressed := (control & 0x80) != 0
		length := int(control & 0x7F)

		if length == 0 {
			continue
		}

		if isCompressed {
			if i >= len(data) {
				return result
			}

			value := data[i]
			i++

			start := len(result)
			result = append(result, make([]byte, length)...)
			for j := 0; j < length; j++ {
				result[start+j] = value
			}
		} else {
			if i+length > len(data) {
				return result
			}

			result = append(result, data[i:i+length]...)
			i += length
		}
	}

	return result
}

func (sa *SimpleArchiver) CompressFile(inputPath, outputPath string) error {
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input file %q: %w", inputPath, err)
	}
	defer inputFile.Close()

	createdFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file %q: %w", outputPath, err)
	}
	defer createdFile.Close()

	reader := bufio.NewReader(inputFile)
	writer := bufio.NewWriter(createdFile)
	defer writer.Flush()

	fileName := filepath.Base(inputPath)
	if len(fileName) > 255 {
		return fmt.Errorf("file name too long for header: %q", fileName)
	}

	if _, err = writer.Write([]byte{byte(len(fileName))}); err != nil {
		return fmt.Errorf("write file name length to archive %q: %w", outputPath, err)
	}

	if _, err = writer.Write([]byte(fileName)); err != nil {
		return fmt.Errorf("write file name %q to archive %q: %w", fileName, outputPath, err)
	}

	for {
		n, readErr := reader.Read(sa.buffer)

		if n > 0 {
			compressed := sa.compress(sa.buffer[:n])

			if len(compressed) > 0xFFFF {
				return fmt.Errorf("compressed block too large: %d bytes", len(compressed))
			}

			blockSize := uint16(len(compressed))

			if err = writer.WriteByte(byte(blockSize >> 8)); err != nil {
				return fmt.Errorf("write compressed block size high byte: %w", err)
			}
			if err = writer.WriteByte(byte(blockSize)); err != nil {
				return fmt.Errorf("write compressed block size low byte: %w", err)
			}

			if _, err = writer.Write(compressed); err != nil {
				return fmt.Errorf("write compressed block data: %w", err)
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read input file %q: %w", inputPath, readErr)
		}
	}

	return nil
}

func (sa *SimpleArchiver) DecompressFile(inputPath, outputDir string) error {
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input file %q: %w", inputPath, err)
	}
	defer inputFile.Close()

	reader := bufio.NewReader(inputFile)

	nameLen, err := reader.ReadByte()
	if err != nil {
		return fmt.Errorf("read file name length from archive %q: %w", inputPath, err)
	}

	fileNameBuf := make([]byte, int(nameLen))
	if _, err = io.ReadFull(reader, fileNameBuf); err != nil {
		return fmt.Errorf("read file name from archive %q: %w", inputPath, err)
	}

	fileName := string(fileNameBuf)
	outputPath := filepath.Join(outputDir, fileName)

	if err = os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("created output dir %q: %w", outputDir, err)
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file %q: %w", outputPath, err)
	}
	defer outputFile.Close()

	writer := bufio.NewWriter(outputFile)
	defer writer.Flush()

	sizeBuf := make([]byte, 2)
	blockBuf := make([]byte, 0, DefaultBufferSize)
	for {
		_, readErr := io.ReadFull(reader, sizeBuf)

		if readErr == io.EOF {
			break
		}

		if errors.Is(readErr, io.ErrUnexpectedEOF) || readErr != nil {
			return fmt.Errorf("read block size %q: %w", inputPath, readErr)
		}

		blockSize := int(uint16(sizeBuf[0])<<8 | uint16(sizeBuf[1]))

		if cap(blockBuf) < blockSize {
			blockBuf = make([]byte, blockSize)
		}
		blockBuf = blockBuf[:blockSize]

		if _, err = io.ReadFull(reader, blockBuf); err != nil {
			return fmt.Errorf("read block data from archive %q: %w", inputPath, err)
		}

		decompressed := sa.decompress(blockBuf)

		if _, err = writer.Write(decompressed); err != nil {
			return fmt.Errorf("write decompressed data to %q: %w", outputPath, err)
		}
	}

	return nil
}

type model struct {
	archiver  *SimpleArchiver
	state     string
	inputPath string
	choices   []string
	cursor    int
	err       error
}

func initialModel() model {
	return model{
		archiver: NewArchiver(""),
		state:    "menu",
		choices:  []string{"Сжать файл", "Распаковать файл", "Выход"},
		cursor:   0,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m model) View() string {
	if m.state == "menu" {
		return m.viewMenu()
	}

	return ""
}

func (m model) viewMenu() string {
	var b strings.Builder

	b.WriteString("=== Простой архиватор ===\n\n")

	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		b.WriteString(cursor)
		b.WriteString(" ")
		b.WriteString(choice)
		b.WriteString("\n")
	}

	b.WriteString("\nИспользуйте стрелки для навигации и enter для выбора\n")
	b.WriteString("Нажмите q для выхода\n")

	return b.String()
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
