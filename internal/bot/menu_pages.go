package bot

import (
	"fmt"
	"log"
	"time"

	"github.com/bestmjj/prometheus-telegram-bot/internal/prometheus"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/prometheus/common/model"
)

func (b *BotInstance) mainMenuPage(chatID int64, messageID int) tgbotapi.Chattable {
	menuTitle := "请选择一个主菜单"
	menuItems := []MenuItem{
		{Text: "实例", CallbackData: instanceMenuID},
		{Text: "其他", CallbackData: otherMenuID},
	}
	rows := b.generateMenuRows(menuItems)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if messageID == 0 {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("%s\n", menuTitle))
		msg.ReplyMarkup = keyboard
		msg.ParseMode = "HTML"
		return msg
	} else {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("%s\n", menuTitle))
		editMsg.ReplyMarkup = &keyboard
		editMsg.ParseMode = "HTML"
		return editMsg
	}
}

func (b *BotInstance) instanceMenuPage(chatID int64, messageID int) tgbotapi.Chattable {
	menuTitle := "请选择一个实例子菜单"
	menuItems := []MenuItem{
		{Text: "实例总览", CallbackData: instanceOverviewMenuID},
		{Text: "所有实例", CallbackData: allInstancesMenuID},
		{Text: "在线实例", CallbackData: onlineInstancesMenuID},
		{Text: "离线实例", CallbackData: offlineInstancesMenuID},
		{Text: "返回", CallbackData: b.getPreviousMenuID()},
		{Text: "返回主菜单", CallbackData: mainMenuID},
	}
	rows := b.generateMenuRows(menuItems)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if messageID == 0 {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("%s\n", menuTitle))
		msg.ReplyMarkup = keyboard
		msg.ParseMode = "HTML"
		return msg
	} else {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("%s\n", menuTitle))
		editMsg.ReplyMarkup = &keyboard
		editMsg.ParseMode = "HTML"
		return editMsg
	}
}

func (b *BotInstance) instanceOverviewMenuPage(chatID int64, messageID int) tgbotapi.Chattable {
	instances := b.fetchInstancesForMenu(allInstancesMenuID)
	onlineCount := len(b.fetchInstancesForMenu(onlineInstancesMenuID))
	offlineCount := len(b.fetchInstancesForMenu(offlineInstancesMenuID))
	menuTitle := fmt.Sprintf("<b>实例总览</b>\n\n"+
		"<b>总共实例:</b> <a href=\"%s\">%d</a>\n"+
		"<b>在线实例:</b> <a href=\"%s\">%d</a>\n"+
		"<b>离线实例:</b> <a href=\"%s\">%d</a>\n",
		b.generateCallbackURL(allInstancesMenuID), len(instances),
		b.generateCallbackURL(onlineInstancesMenuID), onlineCount,
		b.generateCallbackURL(offlineInstancesMenuID), offlineCount)
	var instance model.Metric

	now := time.Now()
	// 获取昨日流量
	yesterdayTransmitBytes, yesterdayReceiveBytes, err := b.PrometheusClient.GetYesterdayTraffic(instance, now)
	if err != nil {
		errStr := fmt.Sprintf("Failed to query yesterday traffic: %v", err)
		return tgbotapi.NewMessage(chatID, errStr)
	}
	yesterdayTotalBytes := yesterdayTransmitBytes + yesterdayReceiveBytes

	menuTitle += "\n<b>昨日流量:</b>\n"
	menuTitle += fmt.Sprintf("  上传: %s\n", prometheus.FormatBytes(yesterdayTransmitBytes))
	menuTitle += fmt.Sprintf("  下载: %s\n", prometheus.FormatBytes(yesterdayReceiveBytes))
	menuTitle += fmt.Sprintf("  总共: %s\n", prometheus.FormatBytes(yesterdayTotalBytes))

	transmitBytes, receiveBytes, err := b.PrometheusClient.GetDailyTraffic(instance, now)
	if err != nil {
		errStr := fmt.Sprintf("failed to get daily traffic for all instance %v", err)
		return tgbotapi.NewMessage(chatID, errStr)
	}
	totalBytes := transmitBytes + receiveBytes

	uploadRate, downloadRate, err := b.PrometheusClient.QueryNetworkRate(instance, now)
	if err != nil {
		errStr := fmt.Sprintf("failed to get network rate for all instance %v", err)
		return tgbotapi.NewMessage(chatID, errStr)
	}

	menuTitle += "\n<b>日流量:</b>\n"
	menuTitle += fmt.Sprintf("  上传: %s\n", prometheus.FormatBytes(transmitBytes))
	menuTitle += fmt.Sprintf("  下载: %s\n", prometheus.FormatBytes(receiveBytes))
	menuTitle += fmt.Sprintf("  总共: %s\n", prometheus.FormatBytes(totalBytes))

	naturalMonthTransmitBytes, naturalMonthReceiveBytes, err := b.PrometheusClient.GetNaturalMonthTraffic(instance, now)
	if err != nil {
		errStr := fmt.Sprintf("failed to get monthly traffic for all instance %v", err)
		return tgbotapi.NewMessage(chatID, errStr)
	}

	naturalMonthTotalBytes := naturalMonthTransmitBytes + naturalMonthReceiveBytes

	menuTitle += "\n<b>月流量:</b>\n"
	menuTitle += fmt.Sprintf("  上传: %s\n", prometheus.FormatBytes(naturalMonthTransmitBytes))
	menuTitle += fmt.Sprintf("  下载: %s\n", prometheus.FormatBytes(naturalMonthReceiveBytes))
	menuTitle += fmt.Sprintf("  总共: %s\n", prometheus.FormatBytes(naturalMonthTotalBytes))

	menuTitle += "\n<b>网络速率:</b>\n"
	menuTitle += fmt.Sprintf("  上传: %s\n", prometheus.FormatBytesPerSecond(uploadRate))
	menuTitle += fmt.Sprintf("  下载: %s\n", prometheus.FormatBytesPerSecond(downloadRate))

	cpuUsage, memoryUsage, diskUsage, _, _, _, _, err := b.PrometheusClient.FetchResourceMetrics(model.Metric{}, "10m", now)
	if err != nil {
		log.Printf("failed to get resource metrics: %v", err)
	}
	menuTitle += "\n<b>资源使用情况:</b>\n"
	menuTitle += fmt.Sprintf("  CPU 使用率: %.2f%%\n", cpuUsage)
	menuTitle += fmt.Sprintf("  内存使用率: %.2f%%\n", memoryUsage)
	menuTitle += fmt.Sprintf("  磁盘使用率: %.2f%%\n", diskUsage)

	menuItems := []MenuItem{
		{Text: "返回", CallbackData: b.getPreviousMenuID()},
		{Text: "返回主菜单", CallbackData: mainMenuID},
	}
	rows := b.generateMenuRows(menuItems)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if messageID == 0 {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("%s\n", menuTitle))
		msg.ReplyMarkup = keyboard
		msg.ParseMode = "HTML"
		msg.DisableWebPagePreview = true
		return msg
	} else {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("%s\n", menuTitle))
		editMsg.ReplyMarkup = &keyboard
		editMsg.ParseMode = "HTML"
		editMsg.DisableWebPagePreview = true
		return editMsg
	}
}

func (b *BotInstance) allInstancesMenuPage(chatID int64, messageID int, page int) tgbotapi.Chattable {
	instances := b.fetchInstancesForMenu(allInstancesMenuID)
	startIndex := (page - 1) * b.PageSize
	endIndex := startIndex + b.PageSize
	maxInstance := len(instances)
	menuTitle := fmt.Sprintf("请选择一个实例(%d)", maxInstance)
	if endIndex > maxInstance {
		endIndex = maxInstance
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for i := startIndex; i < endIndex; i++ {
		instanceName := string(instances[i]["instance"])
		button := tgbotapi.NewInlineKeyboardButtonData(instanceName, instanceName)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(button))
	}
	if page > 1 {
		prevButton := tgbotapi.NewInlineKeyboardButtonData("上一页", fmt.Sprintf("prev_%s_%d", allInstancesMenuID, page-1))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(prevButton))
	}
	if endIndex < maxInstance {
		nextButton := tgbotapi.NewInlineKeyboardButtonData("下一页", fmt.Sprintf("next_%s_%d", allInstancesMenuID, page+1))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(nextButton))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("返回", instanceMenuID),
		tgbotapi.NewInlineKeyboardButtonData("返回主菜单", mainMenuID)))
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if messageID == 0 {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("%s\n", menuTitle))
		msg.ReplyMarkup = keyboard
		msg.ParseMode = "HTML"
		return msg
	} else {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("%s\n", menuTitle))
		editMsg.ReplyMarkup = &keyboard
		editMsg.ParseMode = "HTML"
		return editMsg
	}
}

func (b *BotInstance) onlineInstancesMenuPage(chatID int64, messageID int, page int) tgbotapi.Chattable {
	instances := b.fetchInstancesForMenu(onlineInstancesMenuID)
	startIndex := (page - 1) * b.PageSize
	endIndex := startIndex + b.PageSize
	maxInstance := len(instances)
	menuTitle := fmt.Sprintf("请选择一个实例(%d)", maxInstance)
	if endIndex > maxInstance {
		endIndex = maxInstance
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for i := startIndex; i < endIndex; i++ {
		instanceName := string(instances[i]["instance"])
		button := tgbotapi.NewInlineKeyboardButtonData(instanceName, instanceName)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(button))
	}
	if page > 1 {
		prevButton := tgbotapi.NewInlineKeyboardButtonData("上一页", fmt.Sprintf("prev_%s_%d", onlineInstancesMenuID, page-1))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(prevButton))
	}
	if endIndex < maxInstance {
		nextButton := tgbotapi.NewInlineKeyboardButtonData("下一页", fmt.Sprintf("next_%s_%d", onlineInstancesMenuID, page+1))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(nextButton))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("返回", instanceMenuID),
		tgbotapi.NewInlineKeyboardButtonData("返回主菜单", mainMenuID)))
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if messageID == 0 {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("%s\n", menuTitle))
		msg.ReplyMarkup = keyboard
		msg.ParseMode = "HTML"
		return msg
	} else {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("%s\n", menuTitle))
		editMsg.ReplyMarkup = &keyboard
		editMsg.ParseMode = "HTML"
		return editMsg
	}
}

func (b *BotInstance) offlineInstancesMenuPage(chatID int64, messageID int, page int) tgbotapi.Chattable {
	instances := b.fetchInstancesForMenu(offlineInstancesMenuID)
	startIndex := (page - 1) * b.PageSize
	endIndex := startIndex + b.PageSize
	maxInstance := len(instances)
	menuTitle := fmt.Sprintf("请选择一个实例(%d)", maxInstance)
	if endIndex > maxInstance {
		endIndex = maxInstance
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for i := startIndex; i < endIndex; i++ {
		instanceName := string(instances[i]["instance"])
		button := tgbotapi.NewInlineKeyboardButtonData(instanceName, instanceName)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(button))
	}
	if page > 1 {
		prevButton := tgbotapi.NewInlineKeyboardButtonData("上一页", fmt.Sprintf("prev_%s_%d", offlineInstancesMenuID, page-1))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(prevButton))
	}
	if endIndex < maxInstance {
		nextButton := tgbotapi.NewInlineKeyboardButtonData("下一页", fmt.Sprintf("next_%s_%d", offlineInstancesMenuID, page+1))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(nextButton))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("返回", instanceMenuID),
		tgbotapi.NewInlineKeyboardButtonData("返回主菜单", mainMenuID)))
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if messageID == 0 {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("%s\n", menuTitle))
		msg.ReplyMarkup = keyboard
		msg.ParseMode = "HTML"
		return msg
	} else {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("%s\n", menuTitle))
		editMsg.ReplyMarkup = &keyboard
		editMsg.ParseMode = "HTML"
		return editMsg
	}
}

func (b *BotInstance) otherMenuPage(chatID int64, messageID int) tgbotapi.Chattable {
	menuTitle := "请选择一个其他子菜单"
	menuItems := []MenuItem{
		{Text: "返回", CallbackData: b.getPreviousMenuID()},
		{Text: "返回主菜单", CallbackData: mainMenuID},
	}
	rows := b.generateMenuRows(menuItems)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if messageID == 0 {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("%s\n", menuTitle))
		msg.ReplyMarkup = keyboard
		msg.ParseMode = "HTML"
		return msg
	} else {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("%s\n", menuTitle))
		editMsg.ReplyMarkup = &keyboard
		editMsg.ParseMode = "HTML"
		return editMsg
	}
}
