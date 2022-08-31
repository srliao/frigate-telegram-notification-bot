package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	badger "github.com/dgraph-io/badger/v3"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type event struct {
	Type   string      `json:"type"`
	Before eventDetail `json:"before"`
	After  eventDetail `json:"after"`
	//custom stuff
	pic   []byte
	ended bool
}

func (b *bot) media(id, media string) ([]byte, error) {
	///api/events/<id>/thumbnail.jpg
	response, err := http.Get(fmt.Sprintf("%v/api/events/%v/%v", b.cfg.FrigateURL, id, media))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return nil, errors.New("received non 200 response code")
	}

	return io.ReadAll(response.Body)
}

type eventDetail struct {
	ID              string      `json:"id"`
	Camera          string      `json:"camera"`
	FrameTime       float64     `json:"frame_time"`
	SnapshotTime    float64     `json:"snapshot_time"`
	Label           string      `json:"label"`
	TopScore        float64     `json:"top_score"`
	FalsePositive   bool        `json:"false_positive"`
	StartTime       float64     `json:"start_time"`
	EndTime         float64     `json:"end_time"`
	Score           float64     `json:"score"`
	Box             []int       `json:"box"`
	Area            int         `json:"area"`
	Region          []int       `json:"region"`
	CurrentZones    []string    `json:"current_zones"`
	EnteredZones    []string    `json:"entered_zones"`
	Thumbnail       interface{} `json:"thumbnail"`
	HasSnapshot     bool        `json:"has_snapshot"`
	HasClip         bool        `json:"has_clip"`
	Stationary      bool        `json:"stationary"`
	MotionlessCount int         `json:"motionless_count"`
	PositionChanges int         `json:"position_changes"`
}

func (b *bot) handleEvents(topics []string, data []byte) {
	var evt event
	err := json.Unmarshal(data, &evt)
	if err != nil {
		log.Printf("error reading event: %v", err)
		return
	}

	//ignore if stationary
	if evt.After.Stationary {
		fmt.Printf("Stationary object %v detect on %v\n", evt.After.Label, evt.After.Camera)
		return
	}

	//ignore if zone count not met
	if count, ok := b.cfg.RequiredZoneCount[evt.After.Camera]; ok {
		if len(evt.After.EnteredZones) < count {
			fmt.Printf("Object %v detected on %v but only entered zones %v\n", evt.After.Label, evt.After.Camera, evt.After.EnteredZones)
			return
		}
	}

	fmt.Printf("%v detect on %v\n", evt.After.Label, evt.After.Camera)

	b.lastEvent = evt.After.ID
	_, ok := b.events[evt.After.ID]
	//if first time grab a picture and send it to the chat
	if !ok {
		if thumb, err := b.media(evt.After.ID, "thumbnail.jpg"); err != nil {
			log.Printf("Error getting thumbnail for id %v: %v\n", evt.After.ID, err)
			b.tb.Send(tgbotapi.NewMessage(
				b.cfg.TelegramChatID,
				fmt.Sprintf("New %v detected on camera %v (id: %v). Sorry I couldn't get a thumbnail :(", evt.After.Label, evt.After.Camera, evt.After.ID),
			))
			evt.pic = thumb
		} else {
			msg, err := b.tb.Send(tgbotapi.NewMessage(
				b.cfg.TelegramChatID,
				fmt.Sprintf("New %v detected on camera %v (id: %v).", evt.After.Label, evt.After.Camera, evt.After.ID),
			))
			if err == nil {
				err = b.db.Update(func(txn *badger.Txn) error {
					e := badger.NewEntry(
						[]byte(strconv.Itoa(msg.MessageID)),
						[]byte(evt.After.ID),
					).WithTTL(time.Hour * 24 * 10)
					err := txn.SetEntry(e)
					return err
				})
				if err != nil {
					log.Printf("Error saving message id into db: %v", err)
				}
			}

			photoFileBytes := tgbotapi.FileBytes{
				Name:  "thumbnail",
				Bytes: thumb,
			}
			photo := tgbotapi.NewPhoto(b.cfg.TelegramChatID, photoFileBytes)
			msg, err = b.tb.Send(photo)
			if err == nil {
				err = b.db.Update(func(txn *badger.Txn) error {
					e := badger.NewEntry(
						[]byte(strconv.Itoa(msg.MessageID)),
						[]byte(evt.After.ID),
					).WithTTL(time.Hour * 24 * 10)
					err := txn.SetEntry(e)
					return err
				})
				if err != nil {
					log.Printf("Error saving message id into db: %v", err)
				}
			}
		}
	}
	switch evt.Type {
	case "update":
		if ok {
			//update our thumbnail
			if thumb, err := b.media(evt.After.ID, "thumbnail.jpg"); err == nil {
				evt.pic = thumb
			}
		}
	case "end":
		// b.tb.Send(tgbotapi.NewMessage(
		// 	b.cfg.TelegramChatID,
		// 	fmt.Sprintf("Event id %v ended; clip now available", evt.After.ID),
		// ))
		evt.ended = true
	}
	b.events[evt.After.ID] = evt
}

func (b *bot) sendSnapshot(id string, replyTo int) {
	pic, err := b.media(id, "snapshot.jpg")
	if err != nil {
		msg := tgbotapi.NewMessage(b.cfg.TelegramChatID, fmt.Sprintf("Sorry! Error occured grabbing snapshot for id %v", b.lastEvent))
		msg.ReplyToMessageID = replyTo
		b.tb.Send(msg)
		return
	}
	photoFileBytes := tgbotapi.FileBytes{
		Name:  "snapshot",
		Bytes: pic,
	}
	photo := tgbotapi.NewPhoto(b.cfg.TelegramChatID, photoFileBytes)
	msg, err := b.tb.Send(photo)
	if err == nil {
		err = b.db.Update(func(txn *badger.Txn) error {
			e := badger.NewEntry(
				[]byte(strconv.Itoa(msg.MessageID)),
				[]byte(id),
			).WithTTL(time.Hour * 24 * 10)
			err := txn.SetEntry(e)
			return err
		})
		if err != nil {
			log.Printf("Error saving message id into db: %v", err)
		}
	}
}

func (b *bot) sendClip(id string, replyTo int) {
	vid, err := b.media(id, "clip.mp4")
	if err != nil {
		msg := tgbotapi.NewMessage(b.cfg.TelegramChatID, fmt.Sprintf("Sorry! Error occured grabbing snapshot for id %v", b.lastEvent))
		msg.ReplyToMessageID = replyTo
		b.tb.Send(msg)
		return
	}
	vidBytes := tgbotapi.FileBytes{
		Name:  "clip",
		Bytes: vid,
	}
	video := tgbotapi.NewVideo(b.cfg.TelegramChatID, vidBytes)
	msg, err := b.tb.Send(video)
	if err == nil {
		err = b.db.Update(func(txn *badger.Txn) error {
			e := badger.NewEntry(
				[]byte(strconv.Itoa(msg.MessageID)),
				[]byte(id),
			).WithTTL(time.Hour * 24 * 10)
			err := txn.SetEntry(e)
			return err
		})
		if err != nil {
			log.Printf("Error saving message id into db: %v", err)
		}
	}
}
