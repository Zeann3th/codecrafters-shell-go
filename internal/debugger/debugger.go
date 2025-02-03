package debuggger

import (
	"fmt"
	"os"
	"runtime"
)

type Debugger struct {
	enabled bool
}

// Creates a Debugger instance, if enabled, it will log out to the console; it will always log to file 'debug.log'
func NewDebugger() *Debugger {
	d := &Debugger{enabled: false}
	return d
}

// Debugger Enabled
func (d *Debugger) Enable() {
	d.enabled = true
}

// Debugger Disabled
func (d *Debugger) Disable() {
	d.enabled = false
}

// Debugger Log
func (d *Debugger) Log(args ...interface{}) {
	file, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("[Debug]: Error opening log file:", err)
		return
	}
	defer file.Close()

	_, filename, line, _ := runtime.Caller(2)
	location := fmt.Sprintf("[Debug]: Log in [%v:%v]:", filename, line)

	var logMessage string
	if len(args) == 0 {
		logMessage = location + "\n"
	} else {
		logMessage = location
		for _, arg := range args {
			logMessage += fmt.Sprintf(" %v", arg)
		}
		logMessage += "\n"
	}

	if _, err := file.WriteString(logMessage); err != nil {
		fmt.Println("[Debug]: Error writing to log file:", err)
	}

	if d.enabled {
		fmt.Print(logMessage)
	}
}

// Util func for debugger, converts the addr of writer to the correct human-readable stdin, stdout, stderr
func GetWriterType(file *os.File) string {
	switch file {
	case os.Stdin:
		return "stdin"
	case os.Stdout:
		return "stdout"
	case os.Stderr:
		return "stderr"
	default:
		if file == nil {
			return "nil"
		}
		return fmt.Sprintf("file(%s)", file.Name())
	}
}
