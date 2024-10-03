package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please provide the number of logs as an argument")
		os.Exit(1)
	}

	numLogs, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("Invalid number of logs:", err)
		os.Exit(1)
	}

	for i := 1; i <= numLogs; i++ {
		fmt.Printf("Log message %d\n", i)
	}
}
