package main

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

var (
	lib     map[string]func(args []string)
	aliases map[string]string
)

func init() {
	lib = map[string]func(args []string){
		"exit":  exit,
		"echo":  echo,
		"type":  _type,
		"pwd":   pwd,
		"cd":    cd,
		"cls":   clear,
		"clear": clear,
		"cat":   cat,
		"help":  help,
	}

	aliases = map[string]string{
		"~": os.Getenv("HOME"),
	}
}

func main() {
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

		op := strings.Fields(command)[0]
		args := strings.Fields(command)[1:]

		if cmd, exists := lib[op]; exists {
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

func exit(args []string) {
	if len(args) > 1 {
		fmt.Println("Error: Expected [0: 1] argument, received " + strconv.Itoa(len(args)))
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

func echo(args []string) {
	for i := range args {
		args[i] = strings.ReplaceAll(args[i], "'", "")
	}
	fmt.Println(strings.Join(args, " "))
	return
}

func _type(args []string) {
	if len(args) != 1 {
		fmt.Println("Error: Expected 1 argument, received " + strconv.Itoa(len(args)))
		return
	} else {
		if _, exists := lib[args[0]]; exists {
			fmt.Println(args[0] + " is a shell builtin")
			return
		} else {
			if fp, exists := find(args[0]); exists {
				fmt.Println(args[0] + " is " + fp)
				return
			}
		}
		fmt.Println(args[0] + ": not found")
		return
	}
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

func pwd(args []string) {
	path, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(path)
	return
}

func clear(args []string) {
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
		fmt.Println("Error: Unsopported OS")
	}
	return
}

func replacePath(path string) string {
	for alias, origin := range aliases {
		path = strings.Replace(path, alias, origin, 1)
	}
	return path
}

func cd(args []string) {
	err := os.Chdir(replacePath(args[0]))
	if err != nil {
		fmt.Printf("cd: %v: No such file or directory\n", args[0])
	}
}

func cat(args []string) {
	var res string
	for _, arg := range args {
		arg = strings.ReplaceAll(arg, "'", "")
		buf, err := os.ReadFile(replacePath(arg))
		if err != nil {
			fmt.Printf("cat: %v: No such file\n", arg)
			return
		}
		res += string(buf)
		res += " "
	}
	fmt.Println(res)
}

func help(args []string) {
	if args[0] == "-a" {
		for command := range lib {
			fmt.Println(command)
		}
	}
	return
}
