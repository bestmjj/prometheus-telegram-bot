package main

import (
	"log"
	"os"
	"strconv"

	"github.com/bestmjj/prometheus-telegram-bot/internal/bot"
	"github.com/bestmjj/prometheus-telegram-bot/internal/prometheus"
)

var (
	prometheusURL string
	botToken      string
	pageSize      int
)

func init() {
	prometheusURL = os.Getenv("PROMETHEUS_URL")
	if prometheusURL == "" {
		log.Fatal("PROMETHEUS_URL environment variable not set")
	}
	botToken = os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN environment variable not set")
	}
	pageSizeStr := os.Getenv("PAGE_SIZE")
	if pageSizeStr == "" {
		pageSize = 5 // Default value if not set
	} else {
		var err error
		pageSize, err = strconv.Atoi(pageSizeStr)
		if err != nil {
			log.Fatalf("PAGE_SIZE is invalid %v", err)
		}
	}
}

func main() {
	prometheusClient, err := prometheus.NewClient(prometheusURL)
	if err != nil {
		log.Fatalf("创建 Prometheus 客户端失败: %v", err)
	}

	botInstance, err := bot.NewBot(botToken, prometheusClient, pageSize)
	if err != nil {
		log.Fatalf("创建 Telegram Bot 失败: %v", err)
	}

	botInstance.Start()
}
