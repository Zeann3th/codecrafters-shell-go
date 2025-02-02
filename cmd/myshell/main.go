package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var lib map[string]func(args []string)

func init() {
	lib = map[string]func(args []string){
		"exit": exit,
		"echo": echo,
		"type": _type,
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
		op := strings.Split(command, " ")[0]
		args := strings.Split(command, " ")[1:]

		if cmd, exists := lib[op]; exists {
			cmd(args)
		} else {
			fmt.Println(command + ": command not found")
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
			fmt.Println(args[0] + ": not found")
			return
		}
	}
}
