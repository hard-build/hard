package main

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	progressReset = "\x1b[0m"
	progressGreen = "\x1b[32m"
	progressCyan  = "\x1b[36m"
)

type progressBar struct {
	mutex         sync.Mutex
	writer        io.Writer
	total         int
	completed     int
	verbose       bool
	silent        bool
	noColor       bool
	previousWidth int
	finished      bool
	err           error
}

func newProgressBar(writer io.Writer, total int, verbose, silent, noColor bool) *progressBar {
	return &progressBar{
		writer:  writer,
		total:   total,
		verbose: verbose,
		silent:  silent,
		noColor: noColor,
	}
}

func (progress *progressBar) complete(source string, diff []byte) {
	progress.mutex.Lock()
	defer progress.mutex.Unlock()
	if progress.finished {
		return
	}

	progress.completed++
	if progress.silent {
		return
	}
	if progress.verbose {
		progress.writeVerboseLocked(source, diff)
		return
	}
	progress.writeNormalLocked(source)
}

func (progress *progressBar) updateStep(source string) {
	progress.mutex.Lock()
	defer progress.mutex.Unlock()
	if progress.finished {
		return
	}
	if progress.completed == 0 {
		progress.completed = 1
	}
	if progress.silent {
		return
	}
	if progress.verbose {
		progress.writeVerboseLocked(source, nil)
		return
	}
	progress.writeNormalLocked(source)
}

func (progress *progressBar) setTotal(total int) {
	progress.mutex.Lock()
	defer progress.mutex.Unlock()
	if !progress.finished {
		progress.total = total
	}
}

func (progress *progressBar) finish() error {
	progress.mutex.Lock()
	defer progress.mutex.Unlock()
	if progress.finished {
		return progress.err
	}
	if !progress.silent && !progress.verbose && progress.previousWidth > 0 {
		progress.writeLocked("\n")
	}
	progress.finished = true
	progress.previousWidth = 0
	return progress.err
}

func (progress *progressBar) writeNormalLocked(source string) {
	plain, rendered := progress.lineLocked(source)
	progress.writeNormalLineLocked(plain, rendered)
}

func (progress *progressBar) writeNormalLineLocked(plain, rendered string) {
	width := utf8.RuneCountInString(plain)
	padding := progress.previousWidth - width
	if padding < 0 {
		padding = 0
	}
	progress.writeLocked("\r" + rendered + strings.Repeat(" ", padding))
	progress.previousWidth = width
}

func (progress *progressBar) writeVerboseLocked(source string, diff []byte) {
	_, rendered := progress.lineLocked(source)
	progress.writeLocked(rendered + "\n")
	if len(diff) != 0 {
		progress.writeLocked(string(diff))
	}
}

func (progress *progressBar) lineLocked(source string) (string, string) {
	prefix := fmt.Sprintf("[%d/%d]", progress.completed, progress.total)
	if progress.total < 0 {
		prefix = fmt.Sprintf("[%d/?]", progress.completed)
	}
	plain := prefix + " " + source
	if progress.noColor {
		return plain, plain
	}
	return plain, progressGreen + prefix + progressReset + " " + progressCyan + source + progressReset
}

func (progress *progressBar) writeLocked(output string) {
	if _, err := io.WriteString(progress.writer, output); err != nil && progress.err == nil {
		progress.err = fmt.Errorf("write progress: %w", err)
	}
}
