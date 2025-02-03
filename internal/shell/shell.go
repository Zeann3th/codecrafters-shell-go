package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	debuggger "github.com/codecrafters-io/shell-starter-go/internal/debugger"
	"golang.org/x/term"
)

// ** Structs **
// ------------------------------------------------------------------------------------------

type CommandFunc func(args []string, next CommandFunc) error

type Shell struct {
	debug    debuggger.Debugger
	stack    []Command
	commands map[string]CommandFunc
	aliases  map[string]string
}

type Command struct {
	op          string
	args        []string
	stdout      string
	stderr      string
	nextCommand *Command
}

type TerminalState struct {
	oldState *term.State
}

// ** Essentials **
// ------------------------------------------------------------------------------------------

// Creates new Shell instance.
// Shell contains builtin commands, aliases for paths, a command stack and a debugger/logger
func NewShell() *Shell {
	s := &Shell{
		debug:    debuggger.Debugger{},
		stack:    []Command{},
		commands: make(map[string]CommandFunc),
		aliases:  map[string]string{"~": os.Getenv("HOME")},
	}
	s.initCommands()
	// s.debug.Enable()
	return s
}

func (s *Shell) Run() {
	termState, err := s.setupTerminal()
	if err != nil {
		fmt.Printf("Error setting up terminal: %v\n", err)
		return
	}
	defer s.restoreTerminal(termState)

	var input strings.Builder
	for {
		fmt.Fprint(os.Stdout, "$ ")

		var buf [1]byte
		for {
			n, err := os.Stdin.Read(buf[:])
			if err != nil || n == 0 {
				continue
			}

			switch buf[0] {
			case 9: // Tab
				completed := s.TabComplete(input.String())
				if completed != input.String() {
					fmt.Print("\r\033[K$ " + completed + " ")
					input.Reset()
					input.WriteString(completed)
				}

			case 13: // Enter
				fmt.Println()
				command := strings.TrimSpace(input.String())
				if command != "" {
					s.parseCommand(command)
					if len(s.stack) > 0 {
						s.executeCommand(s.stack[0])
						s.stack = []Command{}
					}
				}
				input.Reset()
				break

			case 127, 8: // Backspace (Unix) or Backspace (Windows)
				if input.Len() > 0 {
					str := input.String()
					input.Reset()
					input.WriteString(str[:len(str)-1])
					fmt.Print("\b \b")
				}

			case 3: // Ctrl+C
				fmt.Println("\n^C")
				input.Reset()
				break

			case 4: // Ctrl+D
				if input.Len() == 0 {
					fmt.Println("exit")
					os.Exit(0)
				}

			default:
				if buf[0] >= 32 { // Only print printable characters
					input.WriteByte(buf[0])
					fmt.Print(string(buf[0]))
				}
			}

			if buf[0] == 13 { // Enter was pressed
				break
			}
		}
	}
}

// Shell builtin command map
func (s *Shell) initCommands() {
	s.commands["exit"] = s.exit
	s.commands["echo"] = s.echo
	s.commands["type"] = s._type
	s.commands["pwd"] = s.pwd
	s.commands["cd"] = s.cd
	s.commands["cls"] = s.clear
	s.commands["clear"] = s.clear
}

// Shell command parser, parses command into op (operation) and args (arguments for the operation).
// Supports > and &&
func (s *Shell) parseCommand(input string) {
	var current Command
	var current_token strings.Builder
	var singleQuote, doubleQuote, backslash bool
	isFirst := true

	s.stack = []Command{}

	flushToken := func() {
		if current_token.Len() > 0 {
			token := current_token.String()
			if isFirst {
				current.op = token
				isFirst = false
			} else {
				current.args = append(current.args, token)
			}
			current_token.Reset()
		}
	}

	pushCommand := func() {
		flushToken()
		if current.op != "" {
			s.stack = append(s.stack, current)
			current = Command{}
			isFirst = true
		}
	}

	for i := 0; i < len(input); i++ {
		c := rune(input[i])

		switch {
		case backslash:
			current_token.WriteRune(c)
			backslash = false
			continue
		case c == '\\':
			backslash = true
			continue
		case c == '\'':
			if !doubleQuote {
				singleQuote = !singleQuote
				continue
			}
		case c == '"':
			if !singleQuote {
				doubleQuote = !doubleQuote
				continue
			}
		}

		if !singleQuote && !doubleQuote {
			if c == '>' {
				flushToken()
				current.args = append(current.args, ">")
				continue
			}
			if i < len(input)-1 && c == '&' && input[i+1] == '&' {
				pushCommand()
				i++
				continue
			}
		}

		if c == ' ' && !singleQuote && !doubleQuote {
			flushToken()
		} else {
			current_token.WriteRune(c)
		}
	}

	pushCommand()

	for i := 0; i < len(s.stack)-1; i++ {
		s.stack[i].nextCommand = &s.stack[i+1]
	}
}

// Shell generic command execution, contains logic to whether execute builtin or external commands, prints out error if not found
func (s *Shell) executeCommand(cmd Command) error {
	var nextFunc CommandFunc
	if cmd.nextCommand != nil {
		nextFunc = func(args []string, _ CommandFunc) error {
			return s.executeCommand(*cmd.nextCommand)
		}
	}

	s.debug.Log(cmd.op, cmd.args)
	if shellCmd, exists := s.commands[cmd.op]; exists {
		return shellCmd(cmd.args, nextFunc)
	} else if _, exists := find(cmd.op); exists {
		return s.executeExternal(cmd, nextFunc)
	} else {
		fmt.Printf("%s: command not found\n", cmd.op)
		return fmt.Errorf("%s: command not found\n", cmd.op)
	}
}

// Shell external command execution, work in-progress
// TODO: Needs to pipe to  file and not write out to the console if there is '>', '1>', '2>'
func (s *Shell) executeExternal(cmd Command, next CommandFunc) error {
	ext := exec.Command(cmd.op, cmd.args...)
	writer, err := s.pipe(&cmd.args)
	if err != nil {
		return err
	}
	if writer != os.Stdout {
		defer writer.Close()
	}

	ext.Stdout = writer

	err = ext.Run()
	if err != nil {
		return fmt.Errorf("%s: %v", cmd.op, err)
	}

	if next != nil {
		return next(nil, nil)
	}
	return nil
}

// ** Term **
// ------------------------------------------------------------------------------------------

func (s *Shell) setupTerminal() (*TerminalState, error) {
	if runtime.GOOS == "windows" {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return nil, err
		}
		return &TerminalState{oldState: oldState}, nil
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}
	return &TerminalState{oldState: oldState}, nil
}

func (s *Shell) restoreTerminal(ts *TerminalState) {
	if ts != nil && ts.oldState != nil {
		term.Restore(int(os.Stdin.Fd()), ts.oldState)
	}
}

func (s *Shell) TabComplete(input string) string {
	if input == "" {
		return input
	}

	words := strings.Fields(input)
	if len(words) == 0 {
		return input
	}

	if len(words) == 1 && !strings.Contains(input, " ") {
		return s.completeCommand(words[0])
	}

	return s.completePath(input)
}

func (s *Shell) completeCommand(partial string) string {
	matches := []string{}

	// Check built-in commands
	for cmd := range s.commands {
		if strings.HasPrefix(cmd, partial) {
			matches = append(matches, cmd)
		}
	}

	// Check executables in PATH
	if path, exists := find(partial); exists {
		matches = append(matches, filepath.Base(path))
	}

	if len(matches) == 0 {
		return partial
	}

	if len(matches) == 1 {
		return matches[0]
	}

	return s.findCommonPrefix(matches)
}

// ** Builtins **
// ------------------------------------------------------------------------------------------

// Shell builtin exit
func (s *Shell) exit(args []string, next CommandFunc) error {
	if len(args) > 1 {
		return fmt.Errorf("Error: Expected [0:1] argument, received %d", len(args))
	} else if len(args) == 0 {
		os.Exit(0)
	} else {
		code, err := strconv.Atoi(args[0])
		if err != nil {
			return err
		}
		os.Exit(code)
	}
	return nil
}

// Shell builtin pipe, used for external and echo
func (s *Shell) pipe(args *[]string) (*os.File, error) {
	var writer *os.File = os.Stdout

	for i := 0; i < len(*args); i++ {
		if i >= len(*args)-1 {
			break
		}

		if strings.HasPrefix((*args)[i], ">") || strings.HasPrefix((*args)[i], "1>") {
			fi := strings.TrimSpace((*args)[i+1])
			dir := filepath.Dir(fi)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("Error creating directory: %v", err)
			}
			file, err := os.Create(fi)
			if err != nil {
				return nil, fmt.Errorf("Error creating output file: %v", err)
			}
			writer = file

			*args = append((*args)[:i], (*args)[i+2:]...)
			break
		}
	}

	return writer, nil
}

// Shell builtin echo
func (s *Shell) echo(args []string, next CommandFunc) error {
	var output strings.Builder

	writer, err := s.pipe(&args)
	if err != nil {
		return err
	}
	if writer != os.Stdout {
		defer writer.Close()
	}

	output.WriteString(strings.Join(args, " "))
	fmt.Fprintln(writer, output.String())

	if next != nil {
		return next(nil, nil)
	}
	return nil
}

// Shell builtin type, check for builtin or external command
func (s *Shell) _type(args []string, next CommandFunc) error {
	if len(args) != 1 {
		return fmt.Errorf("Error: Expected 1 argument, received %d", len(args))
	}
	if _, exists := s.commands[args[0]]; exists {
		fmt.Println(args[0] + " is a shell builtin")
	} else if fp, exists := find(args[0]); exists {
		fmt.Println(args[0] + " is " + fp)
	} else {
		fmt.Println(args[0] + ": not found")
	}
	if next != nil {
		return next(nil, nil)
	}
	return nil
}

// Shell builtin pwd
func (s *Shell) pwd(args []string, next CommandFunc) error {
	path, err := os.Getwd()
	if err != nil {
		return err
	}
	fmt.Println(path)
	if next != nil {
		return next(nil, nil)
	}
	return nil
}

// Shell builtin clear
func (s *Shell) clear(args []string, next CommandFunc) error {
	switch runtime.GOOS {
	case "linux":
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	default:
		return fmt.Errorf("Error: Unsupported OS")
	}
	if next != nil {
		return next(nil, nil)
	}
	return nil
}

// Shell builtin cd
func (s *Shell) cd(args []string, next CommandFunc) error {
	if len(args) == 0 {
		return fmt.Errorf("Error: No directory specified")
	}
	err := os.Chdir(s.replacePath(args[0]))
	if err != nil {
		return fmt.Errorf("cd: %v: No such file or directory", args[0])
	}
	if next != nil {
		return next(nil, nil)
	}
	return nil
}

// ** Utils **
// ------------------------------------------------------------------------------------------

// Shell path aliases
func (s *Shell) replacePath(path string) string {
	for alias, origin := range s.aliases {
		path = strings.Replace(path, alias, origin, 1)
	}
	return path
}

// Shell executable finder
func find(exe string) (string, bool) {
	paths := strings.Split(os.Getenv("PATH"), ":")
	for _, path := range paths {
		fp := filepath.Join(path, exe)
		if _, err := os.Stat(fp); err == nil {
			return fp, true
		}
	}
	return "NOENT", false
}

// completePath handles file path completion
func (s *Shell) completePath(input string) string {
	lastSpace := strings.LastIndex(input, " ")
	if lastSpace == -1 {
		return input
	}

	prefix := input[:lastSpace+1]
	partial := s.replacePath(input[lastSpace+1:])

	dir := "."
	if filepath.Dir(partial) != "." {
		dir = filepath.Dir(partial)
	}

	pattern := filepath.Join(dir, filepath.Base(partial)+"*")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return input
	}

	if len(matches) == 1 {
		fi, err := os.Stat(matches[0])
		if err != nil {
			return input
		}
		if fi.IsDir() {
			return prefix + matches[0] + string(os.PathSeparator)
		}
		return prefix + matches[0]
	}

	return prefix + s.findCommonPrefix(matches)
}

// findCommonPrefix finds the longest common prefix among strings
func (s *Shell) findCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}

	prefix := strs[0]
	for i := 1; i < len(strs); i++ {
		for !strings.HasPrefix(strs[i], prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}
