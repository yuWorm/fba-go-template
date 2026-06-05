package main

import (
	"context"
	"log"
	"os"

	adminruntime "github.com/yuWorm/fba-go-template/admin/internal/runtime"
)

func main() {
	runtime, err := adminruntime.New()
	if err != nil {
		log.Fatal(err)
	}
	if err := runtime.Execute(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
