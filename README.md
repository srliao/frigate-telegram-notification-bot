# frigate-telegram-notify

This is a Telegram notification bot for Frigate. Meant to serve as an alternative for those who are happy using Frigate as an NVR and would like push notification without running Home Assistant.

The included `docker-compose.yml` will spin up both a MQTT server (Eclipse Mosquitto) to listen to Frigate events as well as a Telegram bot that will subscribe to Frigate events and send the events to a group chat.

Note that the bot will only send and respond to message on the provided group chat id. Group chat allows you to add your family members to the chat so that they can view the events as well.

## Install instructions (Docker Compose)

You will first need a Telegram bot [token](https://core.telegram.org/bots) as well as your group chat [id](https://stackoverflow.com/a/38388851). Then create a `.env` file in the root folder with the following information:

```
MOSQUITTO_USERNAME=mosquitto
MOSQUITTO_PASSWORD=mosquitto
FRIGATE_HOST=192.168.100.58
FRIGATE_PORT=5000
TELEGRAM_KEY=XXXX:YYYYY
TELEGRAM_CHAT_ID=-123456
```

Change the values accordingly.

Start the MQTT server and the bot by running `docker compose up -d`.

In your Frigate's configuration, make sure to point the MQTT server to this server with the appropriate IP, port (1883), and username/password.

That's it! Enjoy
