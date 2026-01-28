package bot

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bestmjj/prometheus-telegram-bot/internal/prometheus"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/prometheus/common/model"
)

func (b *BotInstance) mainMenuPage(chatID int64, messageID int) tgbotapi.Chattable {
	menuTitle := "请选择一个主菜单"
	menuItems := []MenuItem{
		{Text: "实例", CallbackData: instanceMenuID},
		{Text: "实例详情", CallbackData: instanceDetailTableMenuID}, // 添加新菜单项
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

	var menuTitle string

	// 添加标题
	menuTitle += "<b>实例总览</b>\n\n"

	menuTitle += fmt.Sprintf(
		"<b>总共实例:</b> %d\n"+
			"<b>在线实例:</b> %d\n"+
			"<b>离线实例:</b> %d\n\n",
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

	menuTitle += "<b>昨日流量:</b>\n"

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
		msg := tgbotapi.NewMessage(chatID, menuTitle)
		msg.ReplyMarkup = keyboard
		msg.ParseMode = "HTML"
		msg.DisableWebPagePreview = true
		return msg
	} else {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuTitle)
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

func (b *BotInstance) instanceDetailTableMenuPage(chatID int64, messageID int, page int) tgbotapi.Chattable {
	instances := b.fetchInstancesForMenu(allInstancesMenuID)

	// 分页逻辑
	// 详情页内容较多，每页只显示1个实例
	detailPageSize := 1
	startIndex := (page - 1) * detailPageSize
	endIndex := startIndex + detailPageSize
	maxInstance := len(instances)

	if startIndex >= maxInstance {
		startIndex = 0
		endIndex = detailPageSize
		page = 1
	}

	if endIndex > maxInstance {
		endIndex = maxInstance
	}

	var tableContent string

	// 添加标题
	tableContent += fmt.Sprintf("<b>实例详情 (%d/%d)</b>\n\n", page, (maxInstance+detailPageSize-1)/detailPageSize)

	for i := startIndex; i < endIndex; i++ {
		instance := instances[i]
		name := string(instance["instance"])

		// 获取规格信息，从Prometheus标签中提取
		specInfo := ""
		if spec, exists := instance["spec"]; exists {
			specInfo = string(spec)
		} else if info, exists := instance["info"]; exists {
			specInfo = string(info)
		} else if job, exists := instance["job"]; exists {
			specInfo = string(job)
		}

		// 格式化实例名称和规格信息
		formattedName := name
		if specInfo != "" {
			formattedName = fmt.Sprintf("%s(%s)", name, specInfo)
		}

		// 获取实例的真实信息
		info, err := b.PrometheusClient.GetInstanceInfo(instance)
		if err != nil {
			log.Printf("Failed to get instance info for %s: %v", name, err)

			// 如果无法获取实例信息，则显示基本的实例信息
			tableContent += fmt.Sprintf(
				"<b>%d. %s</b>\n"+
					"  • 在线时长: 无法获取\n"+
					"  • 续费日期: 无法获取\n"+
					"  • 续费价格: 无法获取\n"+
					"  • 剩余时间: 无法获取\n"+
					"  • 重置日期: 无法获取\n"+
					"  • 重置日流量: 无法获取\n"+
					"  • 日流量: 无法获取\n"+
					"  • 月流量: 无法获取\n"+
					"  • 昨日流量: 无法获取\n"+
					"  • 资源使用: 无法获取\n\n",
				i+1,
				escapeHTML(formattedName),
			)
		} else {
			// 直接解析信息字符串，逐行处理
			lines := strings.Split(info, "\n")

			var onlineDuration, renewalDate, price, remainingTime, resetDate, resourceUsage string
			var resetDayTraffic, dailyTraffic, monthlyTraffic, yesterdayTraffic string

			// 用于跟踪当前正在解析哪个部分
			currentSection := ""
			var resetDayUpload, resetDayDownload, resetDayTotal string
			var dailyUpload, dailyDownload, dailyTotal string
			var monthlyUpload, monthlyDownload, monthlyTotal string
			var yesterdayUpload, yesterdayDownload, yesterdayTotal string

			for _, line := range lines {
				line = strings.TrimSpace(line)

				// 检查是否是章节标题
				if strings.Contains(line, "重置日流量:") {
					currentSection = "resetDay"
					continue
				} else if strings.Contains(line, "昨日流量:") {
					currentSection = "yesterday"
					continue
				} else if strings.Contains(line, "日流量:") {
					currentSection = "daily"
					continue
				} else if strings.Contains(line, "月流量:") {
					currentSection = "monthly"
					continue
				} else if strings.Contains(line, "网络速率:") {
					currentSection = "network"
					continue
				} else if strings.Contains(line, "资源使用情况:") {
					currentSection = "resource"
					continue
				}

				// 解析各个字段
				if strings.Contains(line, "在线时长:") {
					onlineDuration = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
					onlineDuration = strings.ReplaceAll(onlineDuration, "<b>", "")
					onlineDuration = strings.ReplaceAll(onlineDuration, "</b>", "")
				} else if strings.Contains(line, "续费日期:") {
					renewalDate = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
					renewalDate = strings.ReplaceAll(renewalDate, "<b>", "")
					renewalDate = strings.ReplaceAll(renewalDate, "</b>", "")
				} else if strings.Contains(line, "续费价格:") {
					price = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
					price = strings.ReplaceAll(price, "<b>", "")
					price = strings.ReplaceAll(price, "</b>", "")
				} else if strings.Contains(line, "剩余时间:") {
					remainingTime = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
					remainingTime = strings.ReplaceAll(remainingTime, "<b>", "")
					remainingTime = strings.ReplaceAll(remainingTime, "</b>", "")
				} else if strings.Contains(line, "重置日期:") {
					resetDate = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
					resetDate = strings.ReplaceAll(resetDate, "<b>", "")
					resetDate = strings.ReplaceAll(resetDate, "</b>", "")
				} else if strings.Contains(line, "上传:") && currentSection != "" {
					uploadValue := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
					switch currentSection {
					case "resetDay":
						resetDayUpload = uploadValue
					case "daily":
						dailyUpload = uploadValue
					case "monthly":
						monthlyUpload = uploadValue
					case "yesterday":
						yesterdayUpload = uploadValue
					}
				} else if strings.Contains(line, "下载:") && currentSection != "" {
					downloadValue := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
					switch currentSection {
					case "resetDay":
						resetDayDownload = downloadValue
					case "daily":
						dailyDownload = downloadValue
					case "monthly":
						monthlyDownload = downloadValue
					case "yesterday":
						yesterdayDownload = downloadValue
					}
				} else if strings.Contains(line, "总共:") && currentSection != "" {
					totalValue := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
					switch currentSection {
					case "resetDay":
						resetDayTotal = totalValue
					case "daily":
						dailyTotal = totalValue
					case "monthly":
						monthlyTotal = totalValue
					case "yesterday":
						yesterdayTotal = totalValue
					}
				}
			}

			// 组装流量信息
			if resetDayTotal != "" {
				resetDayTraffic = fmt.Sprintf("上传:%s 下载:%s 总共:%s", resetDayUpload, resetDayDownload, resetDayTotal)
			} else {
				resetDayTraffic = "N/A"
			}

			if dailyTotal != "" {
				dailyTraffic = fmt.Sprintf("上传:%s 下载:%s 总共:%s", dailyUpload, dailyDownload, dailyTotal)
			} else {
				dailyTraffic = "N/A"
			}

			if monthlyTotal != "" {
				monthlyTraffic = fmt.Sprintf("上传:%s 下载:%s 总共:%s", monthlyUpload, monthlyDownload, monthlyTotal)
			} else {
				monthlyTraffic = "N/A"
			}

			if yesterdayTotal != "" {
				yesterdayTraffic = fmt.Sprintf("上传:%s 下载:%s 总共:%s", yesterdayUpload, yesterdayDownload, yesterdayTotal)
			} else {
				yesterdayTraffic = "N/A"
			}

			// 提取资源使用情况的详细信息
			cpuUsage := extractField(lines, "CPU 使用率:")
			memoryUsageDetails := extractField(lines, "内存使用率:")

			// 提取内存使用率的百分比部分
			var memoryUsage string
			if idx := strings.Index(memoryUsageDetails, "%"); idx != -1 {
				parts := strings.Split(memoryUsageDetails, "(")
				if len(parts) > 0 {
					memPercentPart := strings.TrimSpace(parts[0])
					if strings.Contains(memPercentPart, ":") {
						memoryUsage = strings.TrimSpace(strings.SplitN(memPercentPart, ":", 2)[1])
					} else {
						memoryUsage = memPercentPart
					}
				}
			}

			if cpuUsage != "" && memoryUsage != "" {
				resourceUsage = fmt.Sprintf("CPU:%s MEM:%s", cpuUsage, memoryUsage)
			} else {
				resourceUsage = "N/A"
			}

			// 如果某些字段没有值，使用默认值
			if onlineDuration == "" {
				onlineDuration = "N/A"
			}
			if renewalDate == "" {
				renewalDate = "N/A"
			}
			if price == "" {
				price = "N/A"
			}
			if remainingTime == "" {
				remainingTime = "N/A"
			}
			if resetDate == "" {
				resetDate = "N/A"
			}
			if resetDayTraffic == "" {
				resetDayTraffic = "N/A"
			}
			if dailyTraffic == "" {
				dailyTraffic = "N/A"
			}
			if monthlyTraffic == "" {
				monthlyTraffic = "N/A"
			}
			if yesterdayTraffic == "" {
				yesterdayTraffic = "N/A"
			}
			if resourceUsage == "" {
				resourceUsage = "N/A"
			}

			// 为每个实例创建一个垂直布局的详细信息块
			tableContent += fmt.Sprintf(
				"<b>%d. %s</b>\n"+
					"  • 在线时长: %s\n"+
					"  • 续费日期: %s\n"+
					"  • 续费价格: %s\n"+
					"  • 剩余时间: %s\n"+
					"  • 重置日期: %s\n"+
					"  • 重置日流量: %s\n"+
					"  • 日流量: %s\n"+
					"  • 月流量: %s\n"+
					"  • 昨日流量: %s\n"+
					"  • 资源使用: %s\n\n",
				i+1,
				escapeHTML(formattedName),
				escapeHTML(onlineDuration),
				escapeHTML(renewalDate),
				escapeHTML(price),
				escapeHTML(remainingTime),
				escapeHTML(resetDate),
				escapeHTML(resetDayTraffic),
				escapeHTML(dailyTraffic),
				escapeHTML(monthlyTraffic),
				escapeHTML(yesterdayTraffic),
				escapeHTML(resourceUsage),
			)
		}
	}

	menuTitle := tableContent

	// 创建菜单项
	var rows [][]tgbotapi.InlineKeyboardButton

	// 添加分页按钮
	if page > 1 {
		prevButton := tgbotapi.NewInlineKeyboardButtonData("上一页", fmt.Sprintf("prev_%s_%d", instanceDetailTableMenuID, page-1))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(prevButton))
	}
	if endIndex < maxInstance {
		nextButton := tgbotapi.NewInlineKeyboardButtonData("下一页", fmt.Sprintf("next_%s_%d", instanceDetailTableMenuID, page+1))
		if page > 1 {
			// 如果有上一页按钮，将下一页按钮放在同一行
			rows[len(rows)-1] = append(rows[len(rows)-1], nextButton)
		} else {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(nextButton))
		}
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("返回", b.getPreviousMenuID()),
		tgbotapi.NewInlineKeyboardButtonData("返回主菜单", mainMenuID),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if messageID == 0 {
		msg := tgbotapi.NewMessage(chatID, menuTitle)
		msg.ReplyMarkup = keyboard
		msg.ParseMode = "HTML"
		msg.DisableWebPagePreview = true
		return msg
	} else {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuTitle)
		editMsg.ReplyMarkup = &keyboard
		editMsg.ParseMode = "HTML"
		editMsg.DisableWebPagePreview = true
		return editMsg
	}
}

func (b *BotInstance) instanceInfoPage(chatID int64, messageID int, instanceName string) tgbotapi.Chattable {
	var selectedInstance model.Metric

	// Search for the instance
	allInstances := b.fetchInstancesForMenu(allInstancesMenuID)
	for _, instance := range allInstances {
		if string(instance["instance"]) == instanceName {
			selectedInstance = instance
			break
		}
	}

	var info string
	if len(selectedInstance) == 0 {
		info = "无效的实例，请重试。"
	} else {
		var err error
		info, err = b.PrometheusClient.GetInstanceInfo(selectedInstance)
		if err != nil {
			info = fmt.Sprintf("获取实例信息失败: %v", err)
		}
	}

	menuItems := []MenuItem{
		{Text: "返回", CallbackData: b.getPreviousMenuID()},
		{Text: "返回主菜单", CallbackData: mainMenuID},
	}
	rows := b.generateMenuRows(menuItems)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if messageID == 0 {
		msg := tgbotapi.NewMessage(chatID, info)
		msg.ReplyMarkup = keyboard
		msg.ParseMode = "HTML"
		return msg
	} else {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, info)
		editMsg.ReplyMarkup = &keyboard
		editMsg.ParseMode = "HTML"
		return editMsg
	}
}

// 辅助函数：从实例信息中提取特定字段的值
func extractField(lines []string, fieldName string) string {
	for i, line := range lines {
		if strings.Contains(line, fieldName) {
			value := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			value = strings.ReplaceAll(value, "<b>", "")
			value = strings.ReplaceAll(value, "</b>", "")
			// 如果字段值为空，可能在下一行
			if value == "" && i+1 < len(lines) {
				nextLine := lines[i+1]
				if strings.HasPrefix(strings.TrimSpace(nextLine), "  ") {
					value = strings.TrimSpace(nextLine)
					value = strings.ReplaceAll(value, "<b>", "")
					value = strings.ReplaceAll(value, "</b>", "")
				}
			}
			return value
		}
	}
	return ""
}

// 辅助函数：从文本中提取特定部分的值
func extractValueFromSection(text, sectionTitle, valueTitle string) string {
	sectionStartIdx := strings.Index(text, sectionTitle)
	if sectionStartIdx == -1 {
		return "N/A"
	}

	sectionEndIdx := strings.Index(text[sectionStartIdx:], "\n\n")
	if sectionEndIdx == -1 {
		sectionEndIdx = len(text)
	} else {
		sectionEndIdx = sectionEndIdx + sectionStartIdx
	}

	section := text[sectionStartIdx:sectionEndIdx]

	valueStartIdx := strings.Index(section, valueTitle)
	if valueStartIdx == -1 {
		return "N/A"
	}

	valueStartIdx += len(valueTitle)
	valueEndIdx := strings.Index(section[valueStartIdx:], "\n")
	if valueEndIdx == -1 {
		valueEndIdx = len(section) - valueStartIdx
	} else {
		valueEndIdx = valueEndIdx
	}

	value := strings.TrimSpace(section[valueStartIdx : valueStartIdx+valueEndIdx])
	return value
}

// 辅助函数：截断字符串以适应表格列宽
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength]
}

// 辅助函数：转义HTML特殊字符
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&")
	s = strings.ReplaceAll(s, "<", "<")
	s = strings.ReplaceAll(s, ">", ">")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
