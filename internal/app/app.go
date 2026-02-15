package app

import (
	"fmt"

	"github.com/buckelew/go-monitor/config"
)

func Run(cfg *config.Config) {
	fmt.Println("app running!")
	fmt.Println(cfg)
}
