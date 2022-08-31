package main

import (
	"encoding/json"
	"io"
	"log"
	"os"

	"github.com/srliao/frigate-telegram-notify/pkg/bot"
	"gopkg.in/yaml.v3"
)

func main() {
	file, err := os.Open("/config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		log.Fatal(err)
	}

	var cfg bot.Config
	err = yaml.Unmarshal([]byte(data), &cfg)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	pretty, _ := json.MarshalIndent(cfg, "", " ")
	log.Println(string(pretty))

	err = bot.Run(cfg)
	if err != nil {
		log.Fatal(err)
	}

}
