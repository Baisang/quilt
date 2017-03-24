package tests

import (
	"bufio"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/quilt/quilt/util"
)

const (
	blockStart     = "```javascript\n"
	blockEnd       = "```\n"
	commentPattern = "^\\[//\\]: # \\((.*)\\)\\W*$"
)

var errUnbalanced = errors.New("unbalanced code blocks")

type readmeParser struct {
	currentBlock string
	// Map block ID to code block.
	codeBlocks map[string]string
	recording  bool
}

func (parser *readmeParser) parse(line string) error {
	isStart := line == blockStart
	isEnd := line == blockEnd
	reComment := regexp.MustCompile(commentPattern)
	match := reComment.FindStringSubmatch(line)
	isComment := len(match) > 0

	if (isStart && parser.recording) || (isEnd && !parser.recording) {
		return errUnbalanced
	}

	switch {
	case isComment:
		parser.currentBlock = match[1]
	case isStart:
		parser.recording = true

		if parser.currentBlock == "" {
			return errors.New("missing code block id")
		}

		if _, ok := parser.codeBlocks[parser.currentBlock]; !ok {
			parser.codeBlocks[parser.currentBlock] = ""
		}
	case isEnd:
		parser.recording = false
		parser.currentBlock = ""
	}

	if parser.recording && !isStart {
		parser.codeBlocks[parser.currentBlock] += line
	}

	return nil
}

func (parser readmeParser) blocks() (map[string]string, error) {
	if parser.recording {
		return nil, errUnbalanced
	}
	return parser.codeBlocks, nil
}

// TestReadme checks that the code snippets in the README compile.
func TestReadme() error {
	f, err := util.Open("./README.md")
	if err != nil {
		return fmt.Errorf("failed to open README: %s", err.Error())
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	parser := readmeParser{}
	parser.codeBlocks = make(map[string]string)

	for scanner.Scan() {
		if err := parser.parse(scanner.Text() + "\n"); err != nil {
			return fmt.Errorf("failed to parse README: %s",
				err.Error())
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read README: %s", err.Error())
	}

	blocks, err := parser.blocks()
	if err != nil {
		return fmt.Errorf("failed to parse README: %s", err.Error())
	}

	for _, block := range blocks {
		if err = checkSpec(block); err != nil {
			return err
		}
	}
	return nil
}

func checkSpec(content string) error {
	specPath := filepath.Join("/tmp", "readme_block.js")
	util.WriteFile(specPath, []byte(content), 0644)
	return testSpec(specPath)
}
