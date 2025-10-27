package prometheus

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type Client struct {
	api promv1.API
}

func NewClient(prometheusURL string) (*Client, error) {
	client, err := api.NewClient(api.Config{
		Address: prometheusURL,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to create Prometheus client: %v", err)
	}
	v1api := promv1.NewAPI(client)
	return &Client{api: v1api}, nil
}

func (c *Client) FetchInstances(query string) ([]model.Metric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, warnings, err := c.api.Query(ctx, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("Failed to query Prometheus: %v", err)
	}
	if len(warnings) > 0 {
		log.Printf("Warnings: %v", warnings)
	}

	var metrics []model.Metric
	for _, res := range result.(model.Vector) {
		metrics = append(metrics, res.Metric)
	}
	return metrics, nil
}

func (c *Client) GetInstanceInfo(labels model.Metric) (string, error) {
	now := time.Now()
	expiryStr := string(labels["expiry"])
	priceStr := string(labels["price"])
	infoStr := string(labels["info"])
	cycleStr := string(labels["cycle"])

	expiryTime, err := time.Parse("2006-01-02", expiryStr)
	if err != nil {
		return "", fmt.Errorf("Failed to parse expiry date: %v", err)
	}
	lastMonthExpiry := calculateLastMonthExpiry(expiryTime, now)
	lastMonthExpiryStr := fmt.Sprintf("%d-%02d-%02d", lastMonthExpiry.Year(), lastMonthExpiry.Month(), lastMonthExpiry.Day())
	daysDiff := calculateDaysDifference(now, lastMonthExpiry)
	duration := formatDuration(time.Duration(daysDiff*24) * time.Hour)

	// 获取重置日流量
	transmitBytes, receiveBytes, err := c.queryTrafficForDuration(labels, duration, now)
	if err != nil {
		return "", fmt.Errorf("Failed to query reset day traffic: %v", err)
	}

	timeLeft := expiryTime.Sub(now)
	yearsLeft, monthsLeft, daysLeft := calculateTimeLeft(timeLeft)

	// 获取启动时长
	bootTime, err := c.queryNodeBootTime(labels, now)
	if err != nil {
		log.Printf("Failed to query boot time: %v", err)
	}

	info := fmt.Sprintf("<b>实例:</b> %s-->%s\n", string(labels["instance"]), infoStr)
	if bootTime != "" {
		info += fmt.Sprintf("<b>在线时长:</b> %s\n", bootTime)
	}

	info += fmt.Sprintf("<b>续费日期:</b> %s\n", expiryStr)
	info += fmt.Sprintf("<b>续费价格:</b> %s(%s)\n", priceStr, cycleStr)
	info += fmt.Sprintf("<b>剩余时间:</b> %d 年 %d 月 %d 天\n", yearsLeft, monthsLeft, daysLeft)
	info += fmt.Sprintf("<b>重置日期:</b> %s\n", lastMonthExpiryStr)

	info += fmt.Sprintf("\n<b>重置日流量:</b>\n")
	info += fmt.Sprintf("  上传: %s\n", FormatBytes(transmitBytes))
	info += fmt.Sprintf("  下载: %s\n", FormatBytes(receiveBytes))
	info += fmt.Sprintf("  总共: %s\n", FormatBytes(receiveBytes+transmitBytes))
	// 获取自然月流量
	naturalMonthTransmitBytes, naturalMonthReceiveBytes, err := c.GetNaturalMonthTraffic(labels, now)
	if err != nil {
		return "", fmt.Errorf("Failed to query natural month traffic: %v", err)
	}
	naturalMonthTotalBytes := naturalMonthTransmitBytes + naturalMonthReceiveBytes
	info += fmt.Sprintf("\n<b>月流量:</b>\n")
	info += fmt.Sprintf("  上传: %s\n", FormatBytes(naturalMonthTransmitBytes))
	info += fmt.Sprintf("  下载: %s\n", FormatBytes(naturalMonthReceiveBytes))
	info += fmt.Sprintf("  总共: %s\n", FormatBytes(naturalMonthTotalBytes))

	// 获取昨日流量
	yesterdayTransmitBytes, yesterdayReceiveBytes, err := c.GetYesterdayTraffic(labels, now)
	if err != nil {
		return "", fmt.Errorf("Failed to query yesterday traffic: %v", err)
	}
	yesterdayTotalBytes := yesterdayTransmitBytes + yesterdayReceiveBytes

	info += "\n<b>昨日流量:</b>\n"
	info += fmt.Sprintf("  上传: %s\n", FormatBytes(yesterdayTransmitBytes))
	info += fmt.Sprintf("  下载: %s\n", FormatBytes(yesterdayReceiveBytes))
	info += fmt.Sprintf("  总共: %s\n", FormatBytes(yesterdayTotalBytes))

	// 获取每日流量
	naturalDailyTransmitBytes, naturalDailyReceiveBytes, err := c.GetDailyTraffic(labels, now)
	if err != nil {
		return "", fmt.Errorf("Failed to query natural daily traffic: %v", err)
	}
	naturalDailyTotalBytes := naturalDailyTransmitBytes + naturalDailyReceiveBytes
	info += "\n<b>日流量:</b>\n"
	info += fmt.Sprintf("  上传: %s\n", FormatBytes(naturalDailyTransmitBytes))
	info += fmt.Sprintf("  下载: %s\n", FormatBytes(naturalDailyReceiveBytes))
	info += fmt.Sprintf("  总共: %s\n", FormatBytes(naturalDailyTotalBytes))

	// 获取网络速率
	uploadRate, downloadRate, err := c.QueryNetworkRate(labels, now)
	if err != nil {
		log.Printf("Failed to query network rate: %v", err)
	}
	info += "\n<b>网络速率:</b>\n"
	info += fmt.Sprintf("  上传: %s\n", FormatBytesPerSecond(uploadRate))
	info += fmt.Sprintf("  下载: %s\n", FormatBytesPerSecond(downloadRate))

	cpuUsage, memoryUsage, diskUsage, diskTotal, diskAvaileble, memTotal, memAvaileble, err := c.FetchResourceMetrics(labels, duration, now)
	if err != nil {
		log.Printf("Failed to fetch resource metrics: %v", err)
	}

	info += "\n<b>资源使用情况:</b>\n"
	info += fmt.Sprintf("  CPU 使用率: %.2f%%\n", cpuUsage)
	info += fmt.Sprintf("  内存使用率: %.2f%%(共: %s,可用: %s)\n", memoryUsage, FormatBytes(memTotal), FormatBytes(memAvaileble))
	info += fmt.Sprintf("  磁盘使用率: %.2f%%(共: %s,可用: %s)\n", diskUsage, FormatBytes(diskTotal), FormatBytes(diskAvaileble))

	return info, nil
}

func (c *Client) QueryPrometheus(query string, queryTime time.Time) (model.Value, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, warnings, err := c.api.Query(ctx, query, queryTime)
	if err != nil {
		return nil, fmt.Errorf("Failed to query Prometheus: %v", err)
	}
	if len(warnings) > 0 {
		log.Printf("Warning from Prometheus: %v", warnings)
	}
	return result, nil
}

func (c *Client) GetFloatFromPromResult(result model.Value) float64 {
	if result.Type() == model.ValVector && result.(model.Vector).Len() > 0 {
		return float64(result.(model.Vector)[0].Value)
	}
	return 0
}

func (c *Client) queryNodeBootTime(labels model.Metric, queryTime time.Time) (string, error) {
	labelMatchers := BuildLabelMatchers(labels)
	bootTimeQuery := fmt.Sprintf(`time() - node_boot_time_seconds{%s}`, labelMatchers)
	bootTimeResult, err := c.QueryPrometheus(bootTimeQuery, queryTime)
	if err != nil {
		return "", fmt.Errorf("Failed to query node boot time: %v", err)
	}

	if bootTimeResult.Type() == model.ValVector && bootTimeResult.(model.Vector).Len() > 0 {
		bootTime := time.Duration(int64(c.GetFloatFromPromResult(bootTimeResult)) * int64(time.Second))
		return formatDuration(bootTime), nil
	}
	return "", nil
}

func (c *Client) queryTrafficForDuration(labels model.Metric, duration string, now time.Time) (transmitBytes float64, receiveBytes float64, err error) {
	labelMatchers := BuildLabelMatchers(labels)
	transmitQuery := ""
	receiveQuery := ""
	if len(labelMatchers) != 0 {
		transmitQuery = fmt.Sprintf(`sum(increase(node_network_transmit_bytes_total{%s, device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[%s]))`, labelMatchers, duration)
		receiveQuery = fmt.Sprintf(`sum(increase(node_network_receive_bytes_total{%s, device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[%s]))`, labelMatchers, duration)
	} else {
		transmitQuery = fmt.Sprintf(`sum(increase(node_network_transmit_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[%s]))`, duration)
		receiveQuery = fmt.Sprintf(`sum(increase(node_network_receive_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[%s]))`, duration)
	}
	transmitResult, err := c.QueryPrometheus(transmitQuery, now)
	if err != nil {
		return 0, 0, fmt.Errorf("Failed to query transmit bytes: %v", err)
	}
	receiveResult, err := c.QueryPrometheus(receiveQuery, now)
	if err != nil {
		return 0, 0, fmt.Errorf("Failed to query receive bytes: %v", err)
	}

	if transmitResult.Type() == model.ValVector && transmitResult.(model.Vector).Len() > 0 {
		transmitBytes = float64(transmitResult.(model.Vector)[0].Value)
	}
	if receiveResult.Type() == model.ValVector && receiveResult.(model.Vector).Len() > 0 {
		receiveBytes = float64(receiveResult.(model.Vector)[0].Value)
	}
	return transmitBytes, receiveBytes, nil
}

func (c *Client) GetDailyTraffic(labels model.Metric, now time.Time) (transmitBytes float64, receiveBytes float64, err error) {
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	durationCurrentDay := fmt.Sprintf("%.0fh", now.Sub(startOfDay).Hours())
	return c.queryTrafficForDuration(labels, durationCurrentDay, now)

}

func (c *Client) QueryRange(query string, now, start, end time.Time, step time.Duration) (result float64, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queryResult, warnings, err := c.api.QueryRange(ctx, query, promv1.Range{
		Start: start,
		End:   end,
		Step:  step, // Step 可以设置为一天来确保只取一个值
	})
	if err != nil {
		return 0, fmt.Errorf("Failed to query yesterday transmit bytes: %v", err)
	}
	if len(warnings) > 0 {
		log.Printf("Warnings: %v", warnings)
	}

	if queryResult.Type() == model.ValMatrix && queryResult.(model.Matrix).Len() > 0 {
		lastValue := queryResult.(model.Matrix)[0].Values[len(queryResult.(model.Matrix)[0].Values)-1]
		result = float64(lastValue.Value)
	}
	return result, nil
}

func (c *Client) GetYesterdayTraffic(labels model.Metric, now time.Time) (float64, float64, error) {
	yesterday := now.Add(-24 * time.Hour)
	startOfYesterday := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
	endOfYesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(-time.Second)
	step := time.Hour * 24 // Step 可以设置为一天来确保只取一个值
	labelMatchers := BuildLabelMatchers(labels)

	transmitQuery := "sum(increase(node_network_transmit_bytes_total{device=~\"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*\"}[1d]))"
	receiveQuery := "sum(increase(node_network_receive_bytes_total{device=~\"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*\"}[1d]))"

	if len(labelMatchers) != 0 {
		transmitQuery = fmt.Sprintf("sum(increase(node_network_transmit_bytes_total{%s, device=~\"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*\"}[1d]))", labelMatchers)
		receiveQuery = fmt.Sprintf("sum(increase(node_network_receive_bytes_total{%s, device=~\"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*\"}[1d]))", labelMatchers)
	}

	transmitResult, err := c.QueryRange(transmitQuery, now, startOfYesterday, endOfYesterday, step)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to query yesterday transmit bytes: %v", err)
	}

	receiveResult, err := c.QueryRange(receiveQuery, now, startOfYesterday, endOfYesterday, step)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to query yesterday transmit bytes: %v", err)
	}

	return transmitResult, receiveResult, nil
}

func (c *Client) GetNaturalMonthTraffic(labels model.Metric, now time.Time) (transmitBytes float64, receiveBytes float64, err error) {
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	durationCurrentMonth := fmt.Sprintf("%.0fh", now.Sub(startOfMonth).Hours())
	return c.queryTrafficForDuration(labels, durationCurrentMonth, now)
}

func (c *Client) FetchResourceMetrics(labels model.Metric, duration string, now time.Time) (cpuUsage, memoryUsage, diskUsage, diskTotal, diskAvaileble, memTotal, memAvaileble float64, err error) {
	labelMatchers := BuildLabelMatchers(labels)
	cpuQuery := fmt.Sprintf(`avg(rate(node_cpu_seconds_total{mode!="idle"}[%s])) * 100`, duration)
	memoryQuery := fmt.Sprintf(`(1 - avg(node_memory_MemAvailable_bytes{}) / avg(node_memory_MemTotal_bytes{}))*100`)
	diskQuery := fmt.Sprintf(`(1 - avg(node_filesystem_avail_bytes{fstype!="rootfs"}) / avg(node_filesystem_size_bytes{fstype!="rootfs"}))*100`)
	diskTotalQuery := fmt.Sprintf(`node_filesystem_size_bytes{fstype!="rootfs",fstype=~"ext4|xfs"}`)
	diskAvailebleQuery := fmt.Sprintf(`node_filesystem_avail_bytes{fstype!="rootfs",fstype=~"ext4|xfs"}`)
	memTotalQuery := fmt.Sprintf(`node_memory_MemTotal_bytes`)
	memAvailebleQuery := fmt.Sprintf(`node_memory_MemAvailable_bytes`)

	if len(labelMatchers) > 0 {
		cpuQuery = fmt.Sprintf(`avg(rate(node_cpu_seconds_total{%s, mode!="idle"}[%s])) * 100`, labelMatchers, duration)
		memoryQuery = fmt.Sprintf(`(1 - avg(node_memory_MemAvailable_bytes{%s}) / avg(node_memory_MemTotal_bytes{%s}))*100`, labelMatchers, labelMatchers)
		diskQuery = fmt.Sprintf(`(1 - avg(node_filesystem_avail_bytes{%s, fstype!="rootfs"}) / avg(node_filesystem_size_bytes{%s, fstype!="rootfs"}))*100`, labelMatchers, labelMatchers)
		diskTotalQuery = fmt.Sprintf(`node_filesystem_size_bytes{%s, fstype!="rootfs",fstype=~"ext4|xfs"}`, labelMatchers)
		diskAvailebleQuery = fmt.Sprintf(`node_filesystem_avail_bytes{%s, fstype!="rootfs",fstype=~"ext4|xfs"}`, labelMatchers)
		memTotalQuery = fmt.Sprintf(`node_memory_MemTotal_bytes{%s}`, labelMatchers)
		memAvailebleQuery = fmt.Sprintf(`node_memory_MemAvailable_bytes{%s}`, labelMatchers)
	}
	
	cpuResult, err := c.QueryPrometheus(cpuQuery, now)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("Failed to query CPU usage: %v", err)
	}

	memoryResult, err := c.QueryPrometheus(memoryQuery, now)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("Failed to query memory usage: %v", err)
	}
	memTotalResult, err := c.QueryPrometheus(memTotalQuery, now)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("Failed to query memory total: %v", err)
	}
	memAvailebleResult, err := c.QueryPrometheus(memAvailebleQuery, now)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("Failed to query memory availeble: %v", err)
	}

	diskResult, err := c.QueryPrometheus(diskQuery, now)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("Failed to query disk usage: %v", err)
	}
	diskTotalResult, err := c.QueryPrometheus(diskTotalQuery, now)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("Failed to query disk total: %v", err)
	}
	diskAvailebleResult, err := c.QueryPrometheus(diskAvailebleQuery, now)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("Failed to query disk availeble: %v", err)
	}

	cpuUsage = c.GetFloatFromPromResult(cpuResult)
	memoryUsage = c.GetFloatFromPromResult(memoryResult)
	diskUsage = c.GetFloatFromPromResult(diskResult)
	diskTotal = c.GetFloatFromPromResult(diskTotalResult)
	diskAvaileble = c.GetFloatFromPromResult(diskAvailebleResult)
	memTotal = c.GetFloatFromPromResult(memTotalResult)
	memAvaileble = c.GetFloatFromPromResult(memAvailebleResult)

	return cpuUsage, memoryUsage, diskUsage, diskTotal, diskAvaileble, memTotal, memAvaileble, nil
}

func (c *Client) QueryNetworkRate(labels model.Metric, now time.Time) (uploadRate float64, downloadRate float64, err error) {
	labelMatchers := BuildLabelMatchers(labels)
	uploadQuery := ""
	downloadQuery := ""
	if len(labelMatchers) > 0 {
		uploadQuery = fmt.Sprintf(`sum(rate(node_network_transmit_bytes_total{%s, device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[1m]))`, labelMatchers)
		downloadQuery = fmt.Sprintf(`sum(rate(node_network_receive_bytes_total{%s, device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[1m]))`, labelMatchers)
	} else {
		uploadQuery = fmt.Sprintf(`sum(rate(node_network_transmit_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[1m]))`)
		downloadQuery = fmt.Sprintf(`sum(rate(node_network_receive_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[1m]))`)
	}

	uploadResult, err := c.QueryPrometheus(uploadQuery, now)
	if err != nil {
		return 0, 0, fmt.Errorf("Failed to query upload rate: %v", err)
	}
	downloadResult, err := c.QueryPrometheus(downloadQuery, now)
	if err != nil {
		return 0, 0, fmt.Errorf("Failed to query download rate: %v", err)
	}
	uploadRate = c.GetFloatFromPromResult(uploadResult)
	downloadRate = c.GetFloatFromPromResult(downloadResult)
	return uploadRate, downloadRate, nil
}

func calculateLastMonthExpiry(expiryTime time.Time, now time.Time) time.Time {
	dayOfMonth := expiryTime.Day()
	year, month, _ := expiryTime.Date()
	_, nowMonth, _ := now.Date()
	for {
		// 推到上一个月
		if month == 1 {
			year--
			month = 12
		} else {
			month--
		}

		if month == nowMonth {
			if nowMonth == 1 {
				year--
				month = 12
			} else {
				month--
			}
		}
		// 获取上一个月的最后一天
		lastDayOfPrevMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
		// 计算上一个月的到期时间
		monthExpiry := time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.Local)
		// 如果上一个月的到期时间的日子大于上一个月的最后一天，使用上一个月的最后一天
		if dayOfMonth > lastDayOfPrevMonth {
			monthExpiry = time.Date(year, month, lastDayOfPrevMonth, 0, 0, 0, 0, time.Local)
		}
		// 如果上一个月的到期时间不在 now 后，返回
		if !monthExpiry.After(now) {
			return monthExpiry
		}
	}
}

func calculateDaysDifference(now, lastMonthExpiry time.Time) int {
	daysDiff := int(math.Floor(now.Sub(lastMonthExpiry).Hours() / 24))
	if daysDiff < 0 {
		daysDiff = -daysDiff
	}
	return daysDiff
}

func BuildLabelMatchers(labels model.Metric) string {
	var matcherStrings []string
	for k, v := range labels {
		if k == "__name__" || k == "expiry" || k == "price" || k == "info" || k == "cycle" || k == "job" || k == "cpu" {
			continue
		}
		matcherStrings = append(matcherStrings, fmt.Sprintf("%s=\"%s\"", k, string(v)))
	}
	result := strings.Join(matcherStrings, ",")
	if strings.HasSuffix(result, ",") {
		result = result[:len(result)-1]
	}
	return result
}

func CalculateTraffic(transmitBytes, receiveBytes float64) (float64, float64, float64) {
	totalBytes := transmitBytes + receiveBytes
	receiveGiB := receiveBytes / (1024 * 1024 * 1024)
	transmitGiB := transmitBytes / (1024 * 1024 * 1024)
	return totalBytes, receiveGiB, transmitGiB
}

func calculateTimeLeft(timeLeft time.Duration) (int, int, int) {
	yearsLeft := int(timeLeft.Hours() / (24 * 365))
	monthsLeft := int((timeLeft.Hours() / (24 * 30)) - (float64(yearsLeft) * 12))
	daysLeft := int(timeLeft.Hours()/24) - (yearsLeft * 365) - (monthsLeft * 30)
	return yearsLeft, monthsLeft, daysLeft
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours >= 24 {
		return fmt.Sprintf("%dd", hours/24)
	} else if minutes > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else if seconds > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	} else {
		return fmt.Sprintf("%dh", hours)
	}
}

func FormatBytesPerSecond(bytesPerSecond float64) string {
	const (
		KB float64 = 1024
		MB float64 = KB * 1024
		GB float64 = MB * 1024
		TB float64 = GB * 1024
	)

	switch {
	case bytesPerSecond >= TB:
		return fmt.Sprintf("%.2f TiB/s", bytesPerSecond/TB)
	case bytesPerSecond >= GB:
		return fmt.Sprintf("%.2f GiB/s", bytesPerSecond/GB)
	case bytesPerSecond >= MB:
		return fmt.Sprintf("%.2f MiB/s", bytesPerSecond/MB)
	case bytesPerSecond >= KB:
		return fmt.Sprintf("%.2f KiB/s", bytesPerSecond/KB)
	default:
		return fmt.Sprintf("%.2f B/s", bytesPerSecond)
	}
}

func FormatBytes(bytes float64) string {
	const (
		KB float64 = 1024
		MB float64 = KB * 1024
		GB float64 = MB * 1024
		TB float64 = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TiB", bytes/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GiB", bytes/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MiB", bytes/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KiB", bytes/KB)
	default:
		return fmt.Sprintf("%.2f B", bytes)
	}
}
