package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	debuggger "github.com/codecrafters-io/shell-starter-go/internal/debugger"
)

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
	return s
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

// Shell REPL (Read Eval Print Loop)
// Waits for user input and place commands on stack if having multiple stages
func (s *Shell) Run() {
	for {
		fmt.Fprint(os.Stdout, "$ ")

		command, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fmt.Print(fmt.Errorf("Error: %v", err))
			os.Exit(1)
		}

		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}

		s.parseCommand(command)
		if len(s.stack) > 0 {
			s.executeCommand(s.stack[0])
			// Clear stack after execution
			s.stack = []Command{}
		}
	}
}

// Shell command parser, parses command into op (operation) and args (arguments for the operation).
// Supports > and &&
func (s *Shell) parseCommand(input string) {
	var current Command
	var current_token strings.Builder
	var singleQuote, doubleQuote, backslash bool
	isFirst := true
	expectRedirectTarget := false

	s.stack = []Command{}

	flushToken := func() {
		if current_token.Len() > 0 {
			token := current_token.String()
			if expectRedirectTarget {
				current.stdout = token
				expectRedirectTarget = false
			} else if token == ">" {
				expectRedirectTarget = true
			} else if isFirst {
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
			expectRedirectTarget = false
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
			// if c == '>' {
			// 	flushToken()
			// 	current_token.WriteRune(c)
			// 	flushToken()
			// 	continue
			// }
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

// Shell external command execution, work in-progress
// TODO: Needs to pipe to  file and not write out to the console if there is '>', '1>', '2>'
func (s *Shell) executeExternal(cmd Command, next CommandFunc) error {
	s.debug.Log(cmd.op, cmd.args)
	ext := exec.Command(cmd.op, cmd.args...)
	writer, err := s.pipe(&cmd.args)
	s.debug.Log("Writer: ", debuggger.GetWriterType(writer))
	if err != nil {
		return err
	}
	if writer != os.Stdout {
		defer writer.Close()
	}

	ext.Stderr = os.Stderr
	ext.Args = append([]string{cmd.op}, cmd.args...)

	out, err := ext.Output()
	if err != nil {
		return fmt.Errorf("%s: %v", cmd.op, err)
	}

	fmt.Fprintln(writer, string(out))

	if next != nil {
		return next(nil, nil)
	}
	return nil
}

// Shell generic command execution, contains logic to whether execute builtin or external commands, prints out error if not found
func (s *Shell) executeCommand(cmd Command) error {
	var nextFunc CommandFunc
	if cmd.nextCommand != nil {
		nextFunc = func(args []string, _ CommandFunc) error {
			return s.executeCommand(*cmd.nextCommand)
		}
	}

	if shellCmd, exists := s.commands[cmd.op]; exists {
		return shellCmd(cmd.args, nextFunc)
	} else if _, exists := find(cmd.op); exists {
		return s.executeExternal(cmd, nextFunc)
	} else {
		fmt.Printf("%s: command not found\n", cmd.op)
		return fmt.Errorf("%s: command not found\n", cmd.op)
	}
}

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

// Shell path aliases
func (s *Shell) replacePath(path string) string {
	for alias, origin := range s.aliases {
		path = strings.Replace(path, alias, origin, 1)
	}
	return path
}
