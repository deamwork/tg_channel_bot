package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
)

func parseCli() (string, bool) {
	var configFile string
	var verbose bool
	flag.BoolVar(&verbose, "v", true, "verbose mode")
	flag.StringVar(&configFile, "c", "", "config file path")
	flag.Parse()
	return configFile, verbose
}

func ListenExit(telegramBot *TelegramBot) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	_ = <-c
	telegramBot.Bot.Stop()
	telegramBot.Database.Close()
	log.Println("Exit.")
}

func main() {
	configPath, _ := parseCli()
	t := TelegramBot{}
	if configPath != "" {
		t.LoadConfig(configPath)
	} else {
		t.LoadConfigFromEnv()
	}
	RunPusher(&t)
	go ListenExit(&t)
	t.Serve()
}
