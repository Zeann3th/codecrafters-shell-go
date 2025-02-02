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

type Shell struct {
	queue    []Command
	commands map[string]func(args []string)
	aliases  map[string]string
}

type Command struct {
	op   string
	args []string
}

func NewShell() *Shell {
	s := &Shell{
		queue:    []Command{},
		commands: make(map[string]func(args []string)),
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

		op, args := s.parse(command)

		if cmd, exists := s.commands[op]; exists {
			cmd(args)
		} else {
			ext := exec.Command(op, args...)
			ext.Stderr = os.Stderr
			ext.Stdout = os.Stdout

			err := ext.Run()
			if err != nil {
				fmt.Println(command + ": command not found")
			}
		}
	}
}

func (s *Shell) parse(command string) (string, []string) {
	var args []string
	var current strings.Builder
	var singleQuote, doubleQuote, backslash bool
	isFirst := true
	var op string

	for _, c := range command {
		switch c {
		case '\'':
			if backslash && doubleQuote {
				current.WriteRune('\\')
			}
			if backslash || doubleQuote {
				current.WriteRune(c)
			} else {
				singleQuote = !singleQuote
			}
			backslash = false
		case '"':
			if backslash || singleQuote {
				current.WriteRune(c)
			} else {
				doubleQuote = !doubleQuote
			}
			backslash = false
		case '\\':
			if backslash || singleQuote {
				current.WriteRune(c)
				backslash = false
			} else {
				backslash = true
			}
		case ' ':
			if backslash && doubleQuote {
				current.WriteRune('\\')
			}
			if backslash || singleQuote || doubleQuote {
				current.WriteRune(c)
			} else if current.Len() > 0 {
				if isFirst {
					op = current.String()
					isFirst = false
				} else {
					args = append(args, current.String())
				}
				current.Reset()
			}
			backslash = false
		default:
			if doubleQuote && backslash {
				current.WriteRune('\\')
			}
			current.WriteRune(c)
			backslash = false
		}
	}

	if current.Len() > 0 {
		if isFirst {
			op = current.String()
		} else {
			args = append(args, current.String())
		}
	}

	return op, args
}

func (s *Shell) exit(args []string) {
	if len(args) > 1 {
		fmt.Println("Error: Expected [0:1] argument, received " + strconv.Itoa(len(args)))
		return
	} else if len(args) == 0 {
		os.Exit(0)
	} else {
		code, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Println(fmt.Errorf("Error: %v", err))
			return
		}
		os.Exit(code)
	}
}

func (s *Shell) echo(args []string) {
	fmt.Println(strings.Join(args, " "))
}

func (s *Shell) _type(args []string) {
	if len(args) != 1 {
		fmt.Println("Error: Expected 1 argument, received " + strconv.Itoa(len(args)))
		return
	}
	if _, exists := s.commands[args[0]]; exists {
		fmt.Println(args[0] + " is a shell builtin")
		return
	}
	if fp, exists := find(args[0]); exists {
		fmt.Println(args[0] + " is " + fp)
		return
	}
	fmt.Println(args[0] + ": not found")
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

func (s *Shell) pwd(args []string) {
	path, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(path)
}

func (s *Shell) clear(args []string) {
	_ = args
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
		fmt.Println("Error: Unsupported OS")
	}
}

func (s *Shell) replacePath(path string) string {
	for alias, origin := range s.aliases {
		path = strings.Replace(path, alias, origin, 1)
	}
	return path
}

func (s *Shell) cd(args []string) {
	if len(args) == 0 {
		fmt.Println("Error: No directory specified")
		return
	}
	err := os.Chdir(s.replacePath(args[0]))
	if err != nil {
		fmt.Printf("cd: %v: No such file or directory\n", args[0])
	}
}
