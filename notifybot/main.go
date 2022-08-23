package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/srliao/frigate-telegram-notify/notifybot/pkg/bot"
)

func main() {
	chatIdStr := os.Getenv("TELEGRAM_CHAT_ID")
	chatId, err := strconv.ParseInt(chatIdStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid chat id %v, cannot parse to int", chatIdStr)
	}

	cfg := bot.Config{
		BrokerURL:      fmt.Sprintf("tcp://%s:%s", os.Getenv("MQTT_HOST"), os.Getenv("MQTT_PORT")),
		BrokerUsername: os.Getenv("MQTT_USERNAME"),
		BrokerPassword: os.Getenv("MQTT_PASSWORD"),
		FrigateURL:     fmt.Sprintf("http://%s:%s", os.Getenv("FRIGATE_HOST"), os.Getenv("FRIGATE_PORT")),
		TelegramToken:  os.Getenv("TELEGRAM_KEY"),
		TelegramChatID: chatId,
	}

	pretty, _ := json.MarshalIndent(cfg, "", " ")
	log.Println(string(pretty))

	err = bot.Run(cfg)
	if err != nil {
		log.Fatal(err)
	}

}
