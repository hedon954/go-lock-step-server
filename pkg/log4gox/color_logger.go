package util

import (
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/log4go"
)

var stdout io.Writer = os.Stdout

/*
foreground       background      color
---------------------------------------
30                40              black
31                41              red
32                42              green
33                43              yellow
34                44              blue
35                45              burgundy
36                46              cyan blue
37                47              white
*/

var (
	levelColor   = []int{30, 30, 32, 37, 37, 33, 31, 34}
	levelStrings = []string{"FNST", "FINE", "DEBG", "TRAC", "INFO", "WARN", "EROR", "CRIT"}
)

const (
	colorSymbol = 0x1B
)

// ConsoleLogWriter is a standard writer that prints to standard output
type ConsoleLogWriter chan *log4go.LogRecord

// NewColorConsoleLogWriter creates a new ConsoleLogWriter
func NewColorConsoleLogWriter() ConsoleLogWriter {
	records := make(ConsoleLogWriter, log4go.LogBufferLength)
	go records.run(stdout)
	return records
}

func (w *ConsoleLogWriter) run(out io.Writer) {
	var timestr string
	var timestrAt int64

	for rec := range *w {
		if at := rec.Created.UnixNano() / 1e9; at != timestrAt {
			timestr, timestrAt = rec.Created.Format("01/02/06 15:04:05"), at
		}
		_, _ = fmt.Fprintf(out, "%c[%dm[%s] [%s] (%s) %s\n%c[0m",
			colorSymbol,
			levelColor[rec.Level],
			timestr,
			levelStrings[rec.Level],
			rec.Source,
			rec.Message,
			colorSymbol)
	}
}
