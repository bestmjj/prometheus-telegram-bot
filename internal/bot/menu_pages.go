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
		"<b>总共实例:</b> %d\n"+
		"<b>在线实例:</b> %d\n"+
		"<b>离线实例:</b> %d\n",
		len(instances), onlineCount, offlineCount)

	now := time.Now()
	var instance model.Metric

	// 获取昨日流量
	yesterdayTransmitBytes, yesterdayReceiveBytes, err := b.PrometheusClient.GetYesterdayTraffic(instance, now)
	if err != nil {
		errStr := fmt.Sprintf("Failed to query yesterday traffic: %v", err)
		return tgbotapi.NewMessage(chatID, errStr)
	}
	yesterdayTotalBytes := yesterdayTransmitBytes + yesterdayReceiveBytes

	menuTitle += "\n<b>昨日流量:</b>\n"
	
	// 查询昨日上传流量最大的实例
	highestUploadInstance, highestUploadValue, err := b.PrometheusClient.GetHighestUploadTrafficInstance(now)
	if err != nil {
		log.Printf("failed to get highest upload traffic instance: %v", err)
		menuTitle += fmt.Sprintf("  上传: %s\n", prometheus.FormatBytes(yesterdayTransmitBytes))
	} else if highestUploadInstance != "" {
		menuTitle += fmt.Sprintf("  上传: %s（最多：%s (%s)）\n", prometheus.FormatBytes(yesterdayTransmitBytes), highestUploadInstance, prometheus.FormatBytes(highestUploadValue))
	} else {
		menuTitle += fmt.Sprintf("  上传: %s\n", prometheus.FormatBytes(yesterdayTransmitBytes))
	}
	
	// 查询昨日下载流量最大的实例
	highestDownloadInstance, highestDownloadValue, err := b.PrometheusClient.GetHighestDownloadTrafficInstance(now)
	if err != nil {
		log.Printf("failed to get highest download traffic instance: %v", err)
		menuTitle += fmt.Sprintf("  下载: %s\n", prometheus.FormatBytes(yesterdayReceiveBytes))
	} else if highestDownloadInstance != "" {
		menuTitle += fmt.Sprintf("  下载: %s（最多：%s (%s)）\n", prometheus.FormatBytes(yesterdayReceiveBytes), highestDownloadInstance, prometheus.FormatBytes(highestDownloadValue))
	} else {
		menuTitle += fmt.Sprintf("  下载: %s\n", prometheus.FormatBytes(yesterdayReceiveBytes))
	}
	
	// 查询昨日总流量最大的实例
	highestTotalInstance, highestTotalValue, err := b.PrometheusClient.GetHighestTotalTrafficInstance(now)
	if err != nil {
		log.Printf("failed to get highest total traffic instance: %v", err)
		menuTitle += fmt.Sprintf("  总共: %s\n", prometheus.FormatBytes(yesterdayTotalBytes))
	} else if highestTotalInstance != "" {
		menuTitle += fmt.Sprintf("  总共: %s（最多：%s (%s)）\n", prometheus.FormatBytes(yesterdayTotalBytes), highestTotalInstance, prometheus.FormatBytes(highestTotalValue))
	} else {
		menuTitle += fmt.Sprintf("  总共: %s\n", prometheus.FormatBytes(yesterdayTotalBytes))
	}

	// Get daily traffic
	transmitBytes, receiveBytes, err := b.PrometheusClient.GetDailyTraffic(instance, now)
	if err != nil {
		errStr := fmt.Sprintf("failed to get daily traffic for all instance %v", err)
		return tgbotapi.NewMessage(chatID, errStr)
	}

	// Get network rates
	uploadRate, downloadRate, err := b.PrometheusClient.QueryNetworkRate(instance, now)
	if err != nil {
		errStr := fmt.Sprintf("failed to get network rate for all instance %v", err)
		return tgbotapi.NewMessage(chatID, errStr)
	}

	// Add daily traffic with highest values
	menuTitle += "\n<b>日流量:</b>\n"
	dailyTotalBytes := transmitBytes + receiveBytes
	
	// Daily upload traffic
	dailyTransmitInstance, dailyTransmitValue, err := b.PrometheusClient.GetHighestDailyUploadTrafficInstance(now)
	if err != nil {
		log.Printf("failed to get highest daily upload traffic instance: %v", err)
		menuTitle += fmt.Sprintf("  上传: %s\n", prometheus.FormatBytes(transmitBytes))
	} else if dailyTransmitInstance != "" {
		menuTitle += fmt.Sprintf("  上传: %s（最多：%s (%s)）\n", prometheus.FormatBytes(transmitBytes), dailyTransmitInstance, prometheus.FormatBytes(dailyTransmitValue))
	} else {
		menuTitle += fmt.Sprintf("  上传: %s\n", prometheus.FormatBytes(transmitBytes))
	}

	// Daily receive traffic
	dailyReceiveInstance, dailyReceiveValue, err := b.PrometheusClient.GetHighestDailyDownloadTrafficInstance(now)
	if err != nil {
		log.Printf("failed to get highest daily download traffic instance: %v", err)
		menuTitle += fmt.Sprintf("  下载: %s\n", prometheus.FormatBytes(receiveBytes))
	} else if dailyReceiveInstance != "" {
		menuTitle += fmt.Sprintf("  下载: %s（最多：%s (%s)）\n", prometheus.FormatBytes(receiveBytes), dailyReceiveInstance, prometheus.FormatBytes(dailyReceiveValue))
	} else {
		menuTitle += fmt.Sprintf("  下载: %s\n", prometheus.FormatBytes(receiveBytes))
	}

	// Daily total traffic
	dailyTotalInstance, dailyTotalValue, err := b.PrometheusClient.GetHighestDailyTotalTrafficInstance(now)
	if err != nil {
		log.Printf("failed to get highest daily total traffic instance: %v", err)
		menuTitle += fmt.Sprintf("  总共: %s\n", prometheus.FormatBytes(dailyTotalBytes))
	} else if dailyTotalInstance != "" {
		menuTitle += fmt.Sprintf("  总共: %s（最多：%s (%s)）\n", prometheus.FormatBytes(dailyTotalBytes), dailyTotalInstance, prometheus.FormatBytes(dailyTotalValue))
	} else {
		menuTitle += fmt.Sprintf("  总共: %s\n", prometheus.FormatBytes(dailyTotalBytes))
	}

	// Get monthly traffic
	naturalMonthTransmitBytes, naturalMonthReceiveBytes, err := b.PrometheusClient.GetNaturalMonthTraffic(instance, now)
	if err != nil {
		errStr := fmt.Sprintf("failed to get monthly traffic for all instance %v", err)
		return tgbotapi.NewMessage(chatID, errStr)
	}

	naturalMonthTotalBytes := naturalMonthTransmitBytes + naturalMonthReceiveBytes

	// Add monthly traffic with highest values
	menuTitle += "\n<b>月流量:</b>\n"
	
	// Monthly upload traffic
	monthlyTransmitInstance, monthlyTransmitValue, err := b.PrometheusClient.GetHighestMonthlyUploadTrafficInstance(now)
	if err != nil {
		log.Printf("failed to get highest monthly upload traffic instance: %v", err)
		menuTitle += fmt.Sprintf("  上传: %s\n", prometheus.FormatBytes(naturalMonthTransmitBytes))
	} else if monthlyTransmitInstance != "" {
		menuTitle += fmt.Sprintf("  上传: %s（最多：%s (%s)）\n", prometheus.FormatBytes(naturalMonthTransmitBytes), monthlyTransmitInstance, prometheus.FormatBytes(monthlyTransmitValue))
	} else {
		menuTitle += fmt.Sprintf("  上传: %s\n", prometheus.FormatBytes(naturalMonthTransmitBytes))
	}

	// Monthly receive traffic
	monthlyReceiveInstance, monthlyReceiveValue, err := b.PrometheusClient.GetHighestMonthlyDownloadTrafficInstance(now)
	if err != nil {
		log.Printf("failed to get highest monthly download traffic instance: %v", err)
		menuTitle += fmt.Sprintf("  下载: %s\n", prometheus.FormatBytes(naturalMonthReceiveBytes))
	} else if monthlyReceiveInstance != "" {
		menuTitle += fmt.Sprintf("  下载: %s（最多：%s (%s)）\n", prometheus.FormatBytes(naturalMonthReceiveBytes), monthlyReceiveInstance, prometheus.FormatBytes(monthlyReceiveValue))
	} else {
		menuTitle += fmt.Sprintf("  下载: %s\n", prometheus.FormatBytes(naturalMonthReceiveBytes))
	}

	// Monthly total traffic
	monthlyTotalInstance, monthlyTotalValue, err := b.PrometheusClient.GetHighestMonthlyTotalTrafficInstance(now)
	if err != nil {
		log.Printf("failed to get highest monthly total traffic instance: %v", err)
		menuTitle += fmt.Sprintf("  总共: %s\n", prometheus.FormatBytes(naturalMonthTotalBytes))
	} else if monthlyTotalInstance != "" {
		menuTitle += fmt.Sprintf("  总共: %s（最多：%s (%s)）\n", prometheus.FormatBytes(naturalMonthTotalBytes), monthlyTotalInstance, prometheus.FormatBytes(monthlyTotalValue))
	} else {
		menuTitle += fmt.Sprintf("  总共: %s\n", prometheus.FormatBytes(naturalMonthTotalBytes))
	}

	// Add network rates with highest values
	menuTitle += "\n<b>网络速率:</b>\n"
	
	// Highest upload rate
	highestUploadRateInstance, highestUploadRateValue, err := b.PrometheusClient.GetHighestUploadRateInstance(now)
	if err != nil {
		log.Printf("failed to get highest upload rate instance: %v", err)
		menuTitle += fmt.Sprintf("  上传: %s/s\n", prometheus.FormatBytesPerSecond(uploadRate))
	} else if highestUploadRateInstance != "" {
		menuTitle += fmt.Sprintf("  上传: %s/s（最多：%s (%s/s)）\n", prometheus.FormatBytesPerSecond(uploadRate), highestUploadRateInstance, prometheus.FormatBytesPerSecond(highestUploadRateValue))
	} else {
		menuTitle += fmt.Sprintf("  上传: %s/s\n", prometheus.FormatBytesPerSecond(uploadRate))
	}

	// Highest download rate
	highestDownloadRateInstance, highestDownloadRateValue, err := b.PrometheusClient.GetHighestDownloadRateInstance(now)
	if err != nil {
		log.Printf("failed to get highest download rate instance: %v", err)
		menuTitle += fmt.Sprintf("  下载: %s/s\n", prometheus.FormatBytesPerSecond(downloadRate))
	} else if highestDownloadRateInstance != "" {
		menuTitle += fmt.Sprintf("  下载: %s/s（最多：%s (%s/s)）\n", prometheus.FormatBytesPerSecond(downloadRate), highestDownloadRateInstance, prometheus.FormatBytesPerSecond(highestDownloadRateValue))
	} else {
		menuTitle += fmt.Sprintf("  下载: %s/s\n", prometheus.FormatBytesPerSecond(downloadRate))
	}

	// Resource metrics with highest values
	cpuUsage, memoryUsage, diskUsage, _, _, _, _, err := b.PrometheusClient.FetchResourceMetrics(model.Metric{}, "10m", now)
	if err != nil {
		log.Printf("failed to get resource metrics: %v", err)
	}
	menuTitle += "\n<b>资源使用情况:</b>\n"
	
	// Highest CPU usage
	highestCpuInstance, highestCpuValue, err := b.PrometheusClient.GetHighestCpuUsageInstance(now)
	if err != nil {
		log.Printf("failed to get highest CPU usage instance: %v", err)
		menuTitle += fmt.Sprintf("  CPU 使用率: %.2f%%\n", cpuUsage)
	} else if highestCpuInstance != "" {
		menuTitle += fmt.Sprintf("  CPU 使用率: %.2f%%（最多：%s (%.2f%%)）\n", cpuUsage, highestCpuInstance, highestCpuValue)
	} else {
		menuTitle += fmt.Sprintf("  CPU 使用率: %.2f%%\n", cpuUsage)
	}
	
	// Highest memory usage
	highestMemoryInstance, highestMemoryValue, err2 := b.PrometheusClient.GetHighestMemoryUsageInstance(now)
	if err2 != nil {
		log.Printf("failed to get highest memory usage instance: %v", err2)
		menuTitle += fmt.Sprintf("  内存使用率: %.2f%%\n", memoryUsage)
	} else if highestMemoryInstance != "" {
		menuTitle += fmt.Sprintf("  内存使用率: %.2f%%（最多：%s (%.2f%%)）\n", memoryUsage, highestMemoryInstance, highestMemoryValue)
	} else {
		menuTitle += fmt.Sprintf("  内存使用率: %.2f%%\n", memoryUsage)
	}
	
	// Highest disk usage
	highestDiskInstance, highestDiskValue, err3 := b.PrometheusClient.GetHighestDiskUsageInstance(now)
	if err3 != nil {
		log.Printf("failed to get highest disk usage instance: %v", err3)
		menuTitle += fmt.Sprintf("  磁盘使用率: %.2f%%\n", diskUsage)
	} else if highestDiskInstance != "" {
		menuTitle += fmt.Sprintf("  磁盘使用率: %.2f%%（最多：%s (%.2f%%)）\n", diskUsage, highestDiskInstance, highestDiskValue)
	} else {
		menuTitle += fmt.Sprintf("  磁盘使用率: %.2f%%\n", diskUsage)
	}

	menuItems := []MenuItem{
		{Text: "全部实例", CallbackData: allInstancesMenuID},
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
