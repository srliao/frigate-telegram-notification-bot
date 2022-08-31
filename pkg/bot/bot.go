package bot

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	badger "github.com/dgraph-io/badger/v3"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Config struct {
	BrokerURL      string `yaml:"broker_url"`
	BrokerUsername string `yaml:"broker_username"`
	BrokerPassword string `yaml:"broker_password"`
	FrigateURL     string `yaml:"frigate_url"`
	TelegramToken  string `yaml:"telegram_token"`
	TelegramChatID int64  `yaml:"telegram_chat_id"`
	DataFolder     string `yaml:"data_folder"`
}

type bot struct {
	tb        *tgbotapi.BotAPI
	client    mqtt.Client
	events    map[string]event
	lastEvent string
	cfg       Config
	db        *badger.DB
}

func Run(cfg Config) error {

	b := &bot{
		cfg:    cfg,
		events: map[string]event{},
	}

	db, err := badger.Open(badger.DefaultOptions(cfg.DataFolder))
	if err != nil {
		return err
	}
	defer db.Close()
	b.db = db

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
			msg := update.Message
			//do not respond if not in the group
			if msg.Chat.ID != b.cfg.TelegramChatID {
				continue
			}
			if !msg.IsCommand() {
				continue
			}
			log.Printf("Message [%s: %v]: %s", update.Message.From.UserName, update.Message.From.ID, update.Message.Text)

			//check if it's a reply
			if msg.ReplyToMessage != nil {
				//grab event id from db
				var key string
				err := b.db.View(func(txn *badger.Txn) error {
					item, err := txn.Get([]byte(strconv.Itoa(msg.ReplyToMessage.MessageID)))
					if err != nil {
						return err
					}
					item.Value(func(val []byte) error {
						key = string(val)
						return nil
					})
					return nil
				})
				if err != nil {
					log.Printf("Error grabbing key from response id: %v", err)
					continue
				}

				switch msg.Command() {
				case "snapshot":
					b.sendLastSnapshot(key, msg.MessageID)
				case "clip":
					b.sendLastClip(key, msg.MessageID)
				}
			}

			//otherwise last id

			evt, ok := b.events[b.lastEvent]
			if !ok {
				next := tgbotapi.NewMessage(b.cfg.TelegramChatID, "No events yet!")
				next.ReplyToMessageID = msg.MessageID
				b.tb.Send(next)
				continue
			}

			switch msg.Command() {
			case "snapshot":
				b.sendLastSnapshot(evt.After.ID, msg.MessageID)
			case "clip":
				if !evt.ended {
					next := tgbotapi.NewMessage(
						b.cfg.TelegramChatID,
						fmt.Sprintf("Event id %v not ended yet; no clip available", b.lastEvent),
					)
					next.ReplyToMessageID = msg.MessageID
					b.tb.Send(next)
					break
				}
				b.sendLastClip(evt.After.ID, msg.MessageID)
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
