package main

import (
	"fmt"
	"time"
)

func main() {
	dateStr := time.Now().Format("20060102-150405")
	fmt.Println(dateStr)
}
