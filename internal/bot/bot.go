package bot

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/bestmjj/prometheus-telegram-bot/internal/prometheus"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/prometheus/common/model"
)

type BotInstance struct {
	BotAPI           *tgbotapi.BotAPI
	PrometheusClient *prometheus.Client
	PageSize         int
	currentMessageID int
	menuStack        []string
}

const (
	mainMenuID             = "main"
	instanceMenuID         = "instance"
	instanceOverviewMenuID = "instance_overview"
	otherMenuID            = "other"
	allInstancesMenuID     = "all_instances"
	onlineInstancesMenuID  = "online_instances"
	offlineInstancesMenuID = "offline_instances"
)

type MenuItem struct {
	Text         string
	CallbackData string
}

func NewBot(token string, prometheusClient *prometheus.Client, pageSize int) (*BotInstance, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("创建 Telegram Bot 失败: %w", err)
	}
	bot.Debug = true
	log.Printf("已授权账户 %s", bot.Self.UserName)

	return &BotInstance{
		BotAPI:           bot,
		PrometheusClient: prometheusClient,
		PageSize:         pageSize,
		menuStack:        []string{mainMenuID},
	}, nil
}

func (b *BotInstance) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.BotAPI.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			b.handleCallback(update.CallbackQuery)
		} else if update.Message != nil {
			if strings.HasPrefix(update.Message.Text, "/start=") {
				parts := strings.Split(update.Message.Text, "=")
				if len(parts) > 1 {
					callbackData := parts[1]
					b.pushMenu(callbackData)
					b.currentMessageID = b.sendMenuPage(update.Message.Chat.ID, 1)
				} else {
					b.currentMessageID = b.sendMenuPage(update.Message.Chat.ID, 1)

				}
				continue
			}
			b.currentMessageID = b.sendMenuPage(update.Message.Chat.ID, 1)

		}
	}
}

func (b *BotInstance) sendMenuPage(chatID int64, page int) int {
	menuID := b.currentMenu()
	msg := b.editMenuPage(chatID, 0, menuID, page)
	if messageID, ok := msg.(tgbotapi.MessageConfig); ok {
		sentMsg, err := b.BotAPI.Send(messageID)
		if err != nil {
			log.Printf("发送菜单失败: %v", err)
			return 0
		}
		return sentMsg.MessageID
	} else {
		editMsg := msg.(tgbotapi.EditMessageTextConfig)
		_, err := b.BotAPI.Request(editMsg)
		if err != nil {
			log.Printf("发送菜单失败: %v", err)
			return 0
		}
		return editMsg.MessageID
	}
}

func (b *BotInstance) editMenuPage(chatID int64, messageID int, menuID string, page int) tgbotapi.Chattable {
	switch menuID {
	case mainMenuID:
		return b.mainMenuPage(chatID, messageID)
	case instanceMenuID:
		return b.instanceMenuPage(chatID, messageID)
	case instanceOverviewMenuID:
		return b.instanceOverviewMenuPage(chatID, messageID)
	case allInstancesMenuID:
		return b.allInstancesMenuPage(chatID, messageID, page)
	case onlineInstancesMenuID:
		return b.onlineInstancesMenuPage(chatID, messageID, page)
	case offlineInstancesMenuID:
		return b.offlineInstancesMenuPage(chatID, messageID, page)
	case otherMenuID:
		return b.otherMenuPage(chatID, messageID)
	default:
		return tgbotapi.NewMessage(chatID, "未知菜单")
	}
}

func (b *BotInstance) handleCallback(callback *tgbotapi.CallbackQuery) {
	data := callback.Data
	chatID := callback.Message.Chat.ID
	messageID := callback.Message.MessageID
	//log.Printf("Callback data %v", data)

	if strings.HasPrefix(data, "prev_") || strings.HasPrefix(data, "next_") {
		parts := strings.Split(data, "_")
		if len(parts) < 3 {
			log.Printf("Invalid page callback data: %v", data)
			return
		}
		page, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			log.Printf("Invalid page number %v from %v", parts[len(parts)-1], data)
			return
		}
		menuID := strings.Join(parts[1:len(parts)-1], "_")
		editMsg := b.editMenuPage(chatID, messageID, menuID, page)
		b.BotAPI.Request(editMsg)
		b.BotAPI.Request(tgbotapi.NewCallback(callback.ID, ""))
		return
	}

	// 检查是否是实例详情的回调数据
	if strings.HasPrefix(data, "instance_detail:") {
		instanceName := strings.TrimPrefix(data, "instance_detail:")
		
		// 查找实例
		var selectedInstance model.Metric
		allInstances := b.fetchInstancesForMenu(allInstancesMenuID)
		for _, instance := range allInstances {
			if string(instance["instance"]) == instanceName {
				selectedInstance = instance
				break
			}
		}
		
		if len(selectedInstance) == 0 {
			b.editMessage(chatID, messageID, "找不到指定的实例，请重试。")
			return
		}

		info, err := b.PrometheusClient.GetInstanceInfo(selectedInstance)
		if err != nil {
			b.editMessage(chatID, messageID, fmt.Sprintf("获取实例信息失败: %v", err))
			return
		}

		msg := tgbotapi.NewMessage(chatID, info)
		msg.ParseMode = "HTML"
		b.BotAPI.Send(msg)
		b.BotAPI.Request(tgbotapi.NewCallback(callback.ID, ""))
		return
	}

	switch data {
	case mainMenuID, instanceMenuID, otherMenuID, instanceOverviewMenuID:
		b.pushMenu(data)
		editMsg := b.editMenuPage(chatID, messageID, data, 1)
		b.BotAPI.Request(editMsg)
		b.BotAPI.Request(tgbotapi.NewCallback(callback.ID, ""))
	case allInstancesMenuID, onlineInstancesMenuID, offlineInstancesMenuID:
		b.pushMenu(data)
		editMsg := b.editMenuPage(chatID, messageID, data, 1)
		b.BotAPI.Request(editMsg)
		b.BotAPI.Request(tgbotapi.NewCallback(callback.ID, ""))
	default:
		var selectedInstance model.Metric

		for _, instance := range b.fetchInstancesForMenu(b.currentMenu()) {
			if string(instance["instance"]) == data {
				selectedInstance = instance
				break
			}
		}
		if len(selectedInstance) == 0 {
			b.editMessage(chatID, messageID, "无效的选择，请重试。")
			return
		}

		info, err := b.PrometheusClient.GetInstanceInfo(selectedInstance)
		if err != nil {
			b.editMessage(chatID, messageID, fmt.Sprintf("获取实例信息失败: %v", err))
			return
		}

		msg := tgbotapi.NewMessage(chatID, info)
		msg.ParseMode = "HTML"
		b.BotAPI.Send(msg)
		b.popMenu()
		// send the menu again
		editMsg := b.editMenuPage(chatID, 0, b.currentMenu(), 1)
		b.BotAPI.Request(editMsg)
		b.BotAPI.Request(tgbotapi.NewCallback(callback.ID, ""))
	}
}

func (b *BotInstance) editMessage(chatID int64, messageID int, text string) {
	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, text)
	editMsg.ParseMode = "HTML"
	b.BotAPI.Request(editMsg)
}

func (b *BotInstance) generateMenuRows(menuItems []MenuItem) [][]tgbotapi.InlineKeyboardButton {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, item := range menuItems {
		button := tgbotapi.NewInlineKeyboardButtonData(item.Text, item.CallbackData)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(button))
	}
	return rows
}

func (b *BotInstance) currentMenu() string {
	if len(b.menuStack) > 0 {
		return b.menuStack[len(b.menuStack)-1]
	}
	return mainMenuID
}

func (b *BotInstance) pushMenu(menuID string) {
	b.menuStack = append(b.menuStack, menuID)
}
func (b *BotInstance) popMenu() string {
	if len(b.menuStack) > 1 {
		b.menuStack = b.menuStack[:len(b.menuStack)-1]
	}
	return b.currentMenu()
}
func (b *BotInstance) getPreviousMenuID() string {
	if len(b.menuStack) > 1 {
		return b.menuStack[len(b.menuStack)-2]
	}
	return mainMenuID
}

func (b *BotInstance) fetchInstancesForMenu(menuID string) []model.Metric {
	var query string
	switch menuID {
	case allInstancesMenuID:
		query = `up{job="node-exporter"}`
	case onlineInstancesMenuID:
		query = `up{job="node-exporter"}==1`
	case offlineInstancesMenuID:
		query = `up{job="node-exporter"}==0`
	default:
		query = `up{job="node-exporter"}`
	}
	instances, err := b.PrometheusClient.FetchInstances(query)
	if err != nil {
		log.Printf("Failed to fetch instance with query %v: %v", query, err)
	}
	return instances
}

func (b *BotInstance) generateCallbackURL(callbackData string) string {
	encodedData := url.QueryEscape(callbackData)
	return fmt.Sprintf("tg://bot?start=%s", encodedData)
}

