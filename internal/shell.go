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
)

type CommandFunc func(args []string, next CommandFunc) error

type Shell struct {
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

func NewShell() *Shell {
	s := &Shell{
		stack:    []Command{},
		commands: make(map[string]CommandFunc),
		aliases:  map[string]string{"~": os.Getenv("HOME")},
	}
	s.initCommands()
	return s
}

func (s *Shell) initCommands() {
	s.commands["exit"] = s.exit
	s.commands["echo"] = s.echo
	s.commands["type"] = s._type
	s.commands["pwd"] = s.pwd
	s.commands["cd"] = s.cd
	s.commands["cls"] = s.clear
	s.commands["clear"] = s.clear
}

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

func (s *Shell) parseCommand(input string) {
	var current Command
	var current_token strings.Builder
	var singleQuote, doubleQuote, backslash bool
	isFirst := true

	s.stack = []Command{}

	flushToken := func() {
		if current_token.Len() > 0 {
			if isFirst {
				current.op = current_token.String()
				isFirst = false
			} else {
				current.args = append(current.args, current_token.String())
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
		fmt.Printf("%s: command not found", cmd.op)
		return fmt.Errorf("%s: command not found\n", cmd.op)
	}
}

func (s *Shell) executeExternal(cmd Command, next CommandFunc) error {
	ext := exec.Command(cmd.op, cmd.args...)

	if cmd.stdout != "" {
		file, err := os.Create(cmd.stdout)
		if err != nil {
			return fmt.Errorf("Error creating output file: %v", err)
		}
		defer file.Close()
		ext.Stdout = file
	} else {
		ext.Stdout = os.Stdout
	}

	if cmd.stderr != "" {
		file, err := os.Create(cmd.stderr)
		if err != nil {
			return fmt.Errorf("Error creating error file: %v", err)
		}
		defer file.Close()
		ext.Stderr = file
	} else {
		ext.Stderr = os.Stderr
	}

	err := ext.Run()
	if err != nil {
		return fmt.Errorf("%s: command not found", cmd.op)
	}

	if next != nil {
		return next(nil, nil)
	}
	return nil
}

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

func (s *Shell) pipe(arg string) (*os.File, error) {
	var writer *os.File = os.Stdout
	if arg != "" {
		file, err := os.Create(arg)
		if err != nil {
			return nil, fmt.Errorf("Error creating output file: %v", err)
		}
		writer = file
	}
	return writer, nil
}

func (s *Shell) echo(args []string, next CommandFunc) error {
	var output strings.Builder
	var redirectionFile string

	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], ">") || strings.HasPrefix(args[i], "1>") || strings.HasPrefix(args[i], "2>") {
			redirectionFile = strings.TrimSpace(args[i+1])
			args = args[:i]
			break
		}
	}

	output.WriteString(strings.Join(args, " "))

	writer, err := s.pipe(redirectionFile)
	if err != nil {
		return err
	}

	fmt.Fprintln(writer, output.String())

	if next != nil {
		return next(nil, nil)
	}
	return nil
}

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

func (s *Shell) replacePath(path string) string {
	for alias, origin := range s.aliases {
		path = strings.Replace(path, alias, origin, 1)
	}
	return path
}
