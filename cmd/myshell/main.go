package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Ensures gofmt doesn't remove the "fmt" import in stage 1 (feel free to remove this!)
var _ = fmt.Fprint

func main() {
	for {
		// Uncomment this block to pass the first stage
		fmt.Fprint(os.Stdout, "$ ")

		// Wait for user input
		command, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fmt.Print(fmt.Errorf("Error: %v", err))
			os.Exit(1)
		}

		command = strings.TrimSpace(command)

		if strings.HasPrefix(command, "exit") {
			args := strings.Split(command, " ")[1:]
			code, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Print(fmt.Errorf("Error: %v", err))
			}
			os.Exit(code)
		} else {
			fmt.Println(command + ": command not found")
		}
	}
}
