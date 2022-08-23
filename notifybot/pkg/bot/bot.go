package bot

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Config struct {
	BrokerURL      string
	BrokerUsername string
	BrokerPassword string
	FrigateURL     string
	TelegramToken  string
	TelegramChatID int64
}

type bot struct {
	tb        *tgbotapi.BotAPI
	client    mqtt.Client
	events    map[string]event
	lastEvent string
	cfg       Config
}

func Run(cfg Config) error {

	b := &bot{
		cfg:    cfg,
		events: map[string]event{},
	}

	//connect to telegram
	tb, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return err
	}
	log.Printf("Telegram authorized on account %s", tb.Self.UserName)
	b.tb = tb

	go b.handleTelegram()

	//connect to mqtt
	opts := mqttOpts(cfg)
	opts.SetDefaultPublishHandler(b.handlePublishedMsgs)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	if token := client.Subscribe("frigate/#", 1, nil); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	b.client = client

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	return nil
}

func mqttOpts(cfg Config) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	opts.SetKeepAlive(60 * time.Second)
	opts.AddBroker(cfg.BrokerURL)
	opts.SetClientID("frigate-notifybot-client")
	if cfg.BrokerUsername != "" {
		opts.SetUsername(cfg.BrokerUsername)
		opts.SetPassword(cfg.BrokerPassword)
	}
	return opts
}

func (b *bot) handleTelegram() {

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.tb.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil { // If we got a message
			//do not respond if not in the group
			if update.Message.Chat.ID != b.cfg.TelegramChatID {
				continue
			}
			if !update.Message.IsCommand() {
				continue
			}
			log.Printf("Message [%s: %v]: %s", update.Message.From.UserName, update.Message.From.ID, update.Message.Text)

			switch update.Message.Command() {
			case "last_thumbnail":
				b.sendLastThumbnail(update.Message.MessageID)
			case "last_snapshot":
				b.sendLastSnapshot(update.Message.MessageID)
			case "last_clip":
				b.sendLastClip(update.Message.MessageID)
			}
		}
	}
}

func (b *bot) handlePublishedMsgs(client mqtt.Client, msg mqtt.Message) {
	topics := strings.Split(msg.Topic(), "/")
	if len(topics) < 2 {
		return
	}
	if topics[0] != "frigate" {
		return
	}

	switch topics[1] {
	case "stats":
	case "events":
		b.handleEvents(topics, msg.Payload())
	default:
		// fmt.Printf("Received from topic %v message; skipped\n", msg.Topic())
	}

}
