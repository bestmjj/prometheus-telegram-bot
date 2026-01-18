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
	resetDayStr := string(labels["reset_day"])
	priceStr := string(labels["price"])
	infoStr := string(labels["info"])
	cycleStr := string(labels["cycle"])

	expiryTime, err := time.Parse("2006-01-02", expiryStr)
	if err != nil {
		return "", fmt.Errorf("Failed to parse expiry date: %v", err)
	}

	var lastResetDate time.Time
	var nextResetDate time.Time
	var resetDateStr string
	var duration string

	if resetDayStr != "" {
		// 如果有固定的重置日，则使用该重置日
		resetDay, err := time.Parse("2006-01-02", resetDayStr)
		if err != nil {
			return "", fmt.Errorf("Failed to parse reset day: %v", err)
		}

		// 从重置日中提取日期
		resetDayOfMonth := resetDay.Day()

		// 计算上一个重置日
		year := now.Year()
		month := now.Month()

		// 首先尝试当前月份的重置日
		currentMonthResetDate := time.Date(year, month, resetDayOfMonth, 0, 0, 0, 0, time.Local)

		if currentMonthResetDate.Before(now) || currentMonthResetDate.Equal(now.Truncate(24*time.Hour)) {
			// 如果当前月的重置日已经过了，上一个重置日就是当前月的重置日
			lastResetDate = currentMonthResetDate
		} else {
			// 如果当前月的重置日还没到，上一个重置日是上个月的重置日
			var lastMonthYear int
			var lastMonthMonth time.Month
			if month-1 < 1 {
				lastMonthYear = year - 1
				lastMonthMonth = 12
			} else {
				lastMonthYear = year
				lastMonthMonth = month - 1
			}
			lastResetDate = time.Date(lastMonthYear, lastMonthMonth, resetDayOfMonth, 0, 0, 0, 0, time.Local)

			// 检查日期有效性
			lastDayOfMonth := time.Date(lastResetDate.Year(), lastResetDate.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
			if resetDayOfMonth > lastDayOfMonth {
				lastResetDate = time.Date(lastResetDate.Year(), lastResetDate.Month(), lastDayOfMonth, 0, 0, 0, 0, time.Local)
			}
		}

		// 计算下一个重置日
		if currentMonthResetDate.After(now) {
			// 如果当前月的重置日还没到，那就是下一个重置日
			nextResetDate = currentMonthResetDate
		} else {
			// 如果当前月的重置日已经过了，那么下个重置日是下个月的重置日
			var nextMonthYear int
			var nextMonthMonth time.Month
			if month+1 > 12 {
				nextMonthYear = year + 1
				nextMonthMonth = 1
			} else {
				nextMonthYear = year
				nextMonthMonth = month + 1
			}
			nextResetDate = time.Date(nextMonthYear, nextMonthMonth, resetDayOfMonth, 0, 0, 0, 0, time.Local)

			// 检查日期有效性
			lastDayOfMonth := time.Date(nextResetDate.Year(), nextResetDate.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
			if resetDayOfMonth > lastDayOfMonth {
				nextResetDate = time.Date(nextResetDate.Year(), nextResetDate.Month(), lastDayOfMonth, 0, 0, 0, 0, time.Local)
			}
		}

		// 计算从上一个重置日到现在的时间差
		daysDiff := calculateDaysDifference(now, lastResetDate)
		duration = formatDuration(time.Duration(daysDiff*24) * time.Hour)
		resetDateStr = fmt.Sprintf("%d-%02d-%02d", nextResetDate.Year(), nextResetDate.Month(), nextResetDate.Day())
	} else {
		// 如果没有固定的重置日，使用到期日作为参考
		expiryDay := expiryTime.Day()
		year := now.Year()
		month := now.Month()

		// 计算上一个重置日（使用到期日的日期作为周期日）
		currentMonthExpiryDate := time.Date(year, month, expiryDay, 0, 0, 0, 0, time.Local)

		if currentMonthExpiryDate.Before(now) || currentMonthExpiryDate.Equal(now.Truncate(24*time.Hour)) {
			// 如果当前月的到期日已经过了，上一个重置日就是当前月的到期日
			lastResetDate = currentMonthExpiryDate
		} else {
			// 如果当前月的到期日还没到，上一个重置日是上个月的对应日期
			var lastMonthYear int
			var lastMonthMonth time.Month
			if month-1 < 1 {
				lastMonthYear = year - 1
				lastMonthMonth = 12
			} else {
				lastMonthYear = year
				lastMonthMonth = month - 1
			}
			lastResetDate = time.Date(lastMonthYear, lastMonthMonth, expiryDay, 0, 0, 0, 0, time.Local)

			// 检查日期有效性
			lastDayOfMonth := time.Date(lastResetDate.Year(), lastResetDate.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
			if expiryDay > lastDayOfMonth {
				lastResetDate = time.Date(lastResetDate.Year(), lastResetDate.Month(), lastDayOfMonth, 0, 0, 0, 0, time.Local)
			}
		}

		// 计算下一个重置日
		if currentMonthExpiryDate.After(now) {
			// 如果当前月的到期日还没到，那就是下一个重置日
			nextResetDate = currentMonthExpiryDate
		} else {
			// 如果当前月的到期日已经过了，那么下个重置日是下个月的到期日
			var nextMonthYear int
			var nextMonthMonth time.Month
			if month+1 > 12 {
				nextMonthYear = year + 1
				nextMonthMonth = 1
			} else {
				nextMonthYear = year
				nextMonthMonth = month + 1
			}
			nextResetDate = time.Date(nextMonthYear, nextMonthMonth, expiryDay, 0, 0, 0, 0, time.Local)

			// 检查日期有效性
			lastDayOfMonth := time.Date(nextResetDate.Year(), nextResetDate.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
			if expiryDay > lastDayOfMonth {
				nextResetDate = time.Date(nextResetDate.Year(), nextResetDate.Month(), lastDayOfMonth, 0, 0, 0, 0, time.Local)
			}
		}

		daysDiff := calculateDaysDifference(now, lastResetDate)
		duration = formatDuration(time.Duration(daysDiff*24) * time.Hour)
		resetDateStr = fmt.Sprintf("%d-%02d-%02d", nextResetDate.Year(), nextResetDate.Month(), nextResetDate.Day())
	}

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
	info += fmt.Sprintf("<b>重置日期:</b> %s\n", resetDateStr)

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

// GetHighestCpuUsageInstance 返回CPU使用率最高的实例名称和使用率值
func (c *Client) GetHighestCpuUsageInstance(now time.Time) (string, float64, error) {
	query := `topk(1, (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m]))) * 100)`
	
	result, err := c.QueryPrometheus(query, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query highest CPU usage instance: %v", err)
	}

	if result.Type() == model.ValVector {
		vector := result.(model.Vector)
		if vector.Len() > 0 {
			// 获取CPU使用率最高的实例
			point := vector[0]
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			return instance, value, nil
		}
	}

	return "", 0, nil
}

// GetHighestMemoryUsageInstance 返回内存使用率最高的实例名称和使用率值
func (c *Client) GetHighestMemoryUsageInstance(now time.Time) (string, float64, error) {
	query := `topk(1, (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100)`

	result, err := c.QueryPrometheus(query, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query highest memory usage instance: %v", err)
	}

	if result.Type() == model.ValVector {
		vector := result.(model.Vector)
		if vector.Len() > 0 {
			// 获取内存使用率最高的实例
			point := vector[0]
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			return instance, value, nil
		}
	}

	return "", 0, nil
}

// GetHighestDiskUsageInstance 返回磁盘使用率最高的实例名称和使用率值
func (c *Client) GetHighestDiskUsageInstance(now time.Time) (string, float64, error) {
	query := `topk(1, (1 - (node_filesystem_avail_bytes / node_filesystem_size_bytes)) * 100)`

	result, err := c.QueryPrometheus(query, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query highest disk usage instance: %v", err)
	}

	if result.Type() == model.ValVector {
		vector := result.(model.Vector)
		if vector.Len() > 0 {
			// 获取磁盘使用率最高的实例
			point := vector[0]
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			return instance, value, nil
		}
	}

	return "", 0, nil
}

// GetHighestUploadRateInstance 返回上传速率最高的实例名称和速率值
func (c *Client) GetHighestUploadRateInstance(now time.Time) (string, float64, error) {
	query := `topk(1, sum by (instance) (rate(node_network_transmit_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[1m])))`

	result, err := c.QueryPrometheus(query, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query highest upload rate instance: %v", err)
	}

	if result.Type() == model.ValVector {
		vector := result.(model.Vector)
		if vector.Len() > 0 {
			// 获取上传速率最高的实例
			point := vector[0]
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			return instance, value, nil
		}
	}

	return "", 0, nil
}

// GetHighestDownloadRateInstance 返回下载速率最高的实例名称和速率值
func (c *Client) GetHighestDownloadRateInstance(now time.Time) (string, float64, error) {
	query := `topk(1, sum by (instance) (rate(node_network_receive_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[1m])))`

	result, err := c.QueryPrometheus(query, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query highest download rate instance: %v", err)
	}

	if result.Type() == model.ValVector {
		vector := result.(model.Vector)
		if vector.Len() > 0 {
			// 获取下载速率最高的实例
			point := vector[0]
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			return instance, value, nil
		}
	}

	return "", 0, nil
}

// GetHighestUploadTrafficInstance returns the instance with the highest upload traffic in the last 24 hours
func (c *Client) GetHighestUploadTrafficInstance(now time.Time) (string, float64, error) {
	query := `topk(1, sum by (instance) (increase(node_network_transmit_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[1d])))`

	result, err := c.QueryPrometheus(query, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query highest upload traffic instance: %v", err)
	}

	if result.Type() == model.ValVector {
		vector := result.(model.Vector)
		if vector.Len() > 0 {
			// 获取上传流量最高的实例
			point := vector[0]
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			return instance, value, nil
		}
	}

	return "", 0, nil
}

// GetHighestDownloadTrafficInstance returns the instance with the highest download traffic in the last 24 hours
func (c *Client) GetHighestDownloadTrafficInstance(now time.Time) (string, float64, error) {
	query := `topk(1, sum by (instance) (increase(node_network_receive_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[1d])))`

	result, err := c.QueryPrometheus(query, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query highest download traffic instance: %v", err)
	}

	if result.Type() == model.ValVector {
		vector := result.(model.Vector)
		if vector.Len() > 0 {
			// 获取下载流量最高的实例
			point := vector[0]
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			return instance, value, nil
		}
	}

	return "", 0, nil
}

// GetHighestTotalTrafficInstance returns the instance with the highest total traffic in the last 24 hours
func (c *Client) GetHighestTotalTrafficInstance(now time.Time) (string, float64, error) {
	// 先查询上传流量
	queryUpload := `sum by (instance) (increase(node_network_transmit_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[1d]))`
	resultUpload, err := c.QueryPrometheus(queryUpload, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query upload traffic for total calculation: %v", err)
	}

	// 先查询下载流量
	queryDownload := `sum by (instance) (increase(node_network_receive_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[1d]))`
	resultDownload, err := c.QueryPrometheus(queryDownload, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query download traffic for total calculation: %v", err)
	}

	// 将两个结果相加，找出总流量最高的实例
	trafficMap := make(map[string]float64)

	if resultUpload.Type() == model.ValVector {
		vector := resultUpload.(model.Vector)
		for _, point := range vector {
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			trafficMap[instance] += value
		}
	}

	if resultDownload.Type() == model.ValVector {
		vector := resultDownload.(model.Vector)
		for _, point := range vector {
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			trafficMap[instance] += value
		}
	}

	// 找出总流量最高的实例
	var highestInstance string
	var highestValue float64
	for instance, totalValue := range trafficMap {
		if totalValue > highestValue {
			highestValue = totalValue
			highestInstance = instance
		}
	}

	return highestInstance, highestValue, nil
}

// GetHighestDailyUploadTrafficInstance returns the instance with the highest upload traffic in the current day
func (c *Client) GetHighestDailyUploadTrafficInstance(now time.Time) (string, float64, error) {
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	durationCurrentDay := fmt.Sprintf("%.0fh", now.Sub(startOfDay).Hours())
	
	query := fmt.Sprintf(`topk(1, sum by (instance) (increase(node_network_transmit_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[%s])))`, durationCurrentDay)

	result, err := c.QueryPrometheus(query, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query highest daily upload traffic instance: %v", err)
	}

	if result.Type() == model.ValVector {
		vector := result.(model.Vector)
		if vector.Len() > 0 {
			// 获取上传流量最高的实例
			point := vector[0]
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			return instance, value, nil
		}
	}

	return "", 0, nil
}

// GetHighestDailyDownloadTrafficInstance returns the instance with the highest download traffic in the current day
func (c *Client) GetHighestDailyDownloadTrafficInstance(now time.Time) (string, float64, error) {
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	durationCurrentDay := fmt.Sprintf("%.0fh", now.Sub(startOfDay).Hours())
	
	query := fmt.Sprintf(`topk(1, sum by (instance) (increase(node_network_receive_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[%s])))`, durationCurrentDay)

	result, err := c.QueryPrometheus(query, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query highest daily download traffic instance: %v", err)
	}

	if result.Type() == model.ValVector {
		vector := result.(model.Vector)
		if vector.Len() > 0 {
			// 获取下载流量最高的实例
			point := vector[0]
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			return instance, value, nil
		}
	}

	return "", 0, nil
}

// GetHighestDailyTotalTrafficInstance returns the instance with the highest total traffic in the current day
func (c *Client) GetHighestDailyTotalTrafficInstance(now time.Time) (string, float64, error) {
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	durationCurrentDay := fmt.Sprintf("%.0fh", now.Sub(startOfDay).Hours())

	// 先查询上传流量
	queryUpload := fmt.Sprintf(`sum by (instance) (increase(node_network_transmit_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[%s]))`, durationCurrentDay)
	resultUpload, err := c.QueryPrometheus(queryUpload, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query upload traffic for daily total calculation: %v", err)
	}

	// 再查询下载流量
	queryDownload := fmt.Sprintf(`sum by (instance) (increase(node_network_receive_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[%s]))`, durationCurrentDay)
	resultDownload, err := c.QueryPrometheus(queryDownload, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query download traffic for daily total calculation: %v", err)
	}

	// 将两个结果相加，找出当日总流量最高的实例
	trafficMap := make(map[string]float64)

	if resultUpload.Type() == model.ValVector {
		vector := resultUpload.(model.Vector)
		for _, point := range vector {
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			trafficMap[instance] += value
		}
	}

	if resultDownload.Type() == model.ValVector {
		vector := resultDownload.(model.Vector)
		for _, point := range vector {
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			trafficMap[instance] += value
		}
	}

	// 找出当日总流量最高的实例
	var highestInstance string
	var highestValue float64
	for instance, totalValue := range trafficMap {
		if totalValue > highestValue {
			highestValue = totalValue
			highestInstance = instance
		}
	}

	return highestInstance, highestValue, nil
}

// GetHighestMonthlyUploadTrafficInstance returns the instance with the highest upload traffic in the current month
func (c *Client) GetHighestMonthlyUploadTrafficInstance(now time.Time) (string, float64, error) {
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	durationCurrentMonth := fmt.Sprintf("%.0fh", now.Sub(startOfMonth).Hours())
	
	query := fmt.Sprintf(`topk(1, sum by (instance) (increase(node_network_transmit_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[%s])))`, durationCurrentMonth)

	result, err := c.QueryPrometheus(query, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query highest monthly upload traffic instance: %v", err)
	}

	if result.Type() == model.ValVector {
		vector := result.(model.Vector)
		if vector.Len() > 0 {
			// 获取上传流量最高的实例
			point := vector[0]
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			return instance, value, nil
		}
	}

	return "", 0, nil
}

// GetHighestMonthlyDownloadTrafficInstance returns the instance with the highest download traffic in the current month
func (c *Client) GetHighestMonthlyDownloadTrafficInstance(now time.Time) (string, float64, error) {
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	durationCurrentMonth := fmt.Sprintf("%.0fh", now.Sub(startOfMonth).Hours())
	
	query := fmt.Sprintf(`topk(1, sum by (instance) (increase(node_network_receive_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[%s])))`, durationCurrentMonth)

	result, err := c.QueryPrometheus(query, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query highest monthly download traffic instance: %v", err)
	}

	if result.Type() == model.ValVector {
		vector := result.(model.Vector)
		if vector.Len() > 0 {
			// 获取下载流量最高的实例
			point := vector[0]
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			return instance, value, nil
		}
	}

	return "", 0, nil
}

// GetHighestMonthlyTotalTrafficInstance returns the instance with the highest total traffic in the current month
func (c *Client) GetHighestMonthlyTotalTrafficInstance(now time.Time) (string, float64, error) {
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	durationCurrentMonth := fmt.Sprintf("%.0fh", now.Sub(startOfMonth).Hours())

	// 先查询上传流量
	queryUpload := fmt.Sprintf(`sum by (instance) (increase(node_network_transmit_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[%s]))`, durationCurrentMonth)
	resultUpload, err := c.QueryPrometheus(queryUpload, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query upload traffic for monthly total calculation: %v", err)
	}

	// 再查询下载流量
	queryDownload := fmt.Sprintf(`sum by (instance) (increase(node_network_receive_bytes_total{device=~"eth.*|ens.*|eno.*|enp.*|enx.*|enX.*|wlan.*|venet.*"}[%s]))`, durationCurrentMonth)
	resultDownload, err := c.QueryPrometheus(queryDownload, now)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to query download traffic for monthly total calculation: %v", err)
	}

	// 将两个结果相加，找出当月总流量最高的实例
	trafficMap := make(map[string]float64)

	if resultUpload.Type() == model.ValVector {
		vector := resultUpload.(model.Vector)
		for _, point := range vector {
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			trafficMap[instance] += value
		}
	}

	if resultDownload.Type() == model.ValVector {
		vector := resultDownload.(model.Vector)
		for _, point := range vector {
			instance := string(point.Metric["instance"])
			value := float64(point.Value)
			trafficMap[instance] += value
		}
	}

	// 找出当月总流量最高的实例
	var highestInstance string
	var highestValue float64
	for instance, totalValue := range trafficMap {
		if totalValue > highestValue {
			highestValue = totalValue
			highestInstance = instance
		}
	}

	return highestInstance, highestValue, nil
}

func calculateLastMonthExpiry(expiryTime time.Time, now time.Time) time.Time {
	expiryDay := expiryTime.Day()
	currentYear := now.Year()
	currentMonth := now.Month()
	currentDay := now.Day()

	// 首先尝试在当前月份查找到期日
	var lastMonthExpiry time.Time

	// 获取当前月有多少天，以防止到期日在当前月中不存在（例如2月30日）
	daysInCurrentMonth := time.Date(currentYear, currentMonth+1, 0, 0, 0, 0, 0, time.Local).Day()

	if currentDay >= expiryDay {
		// 如果当前日期已经过了或等于到期日，那么本次周期的到期日就是本月的到期日
		// 上一个周期的到期日就是本月的到期日
		if expiryDay <= daysInCurrentMonth {
			lastMonthExpiry = time.Date(currentYear, currentMonth, expiryDay, 0, 0, 0, 0, time.Local)
		} else {
			// 如果到期日大于当前月的天数，使用当月最后一天
			lastMonthExpiry = time.Date(currentYear, currentMonth, daysInCurrentMonth, 0, 0, 0, 0, time.Local)
		}

		// 如果这个日期是今天或未来，我们需要找上一个周期的日期
		if lastMonthExpiry.After(now) {
			// 移动到上一个月
			lastMonthExpiry = shiftToPreviousMonth(currentYear, currentMonth, expiryDay)
		}
	} else {
		// 如果当前日期还没到期，那么上一个周期的到期日是在上个月
		lastMonthExpiry = shiftToPreviousMonth(currentYear, currentMonth, expiryDay)
	}

	return lastMonthExpiry
}

// 辅助函数：获取上一个月的对应到期日
func shiftToPreviousMonth(year int, month time.Month, day int) time.Time {
	// 将日期设置为上个月
	prevMonth := month - 1
	prevYear := year
	if prevMonth < 1 {
		prevYear = year - 1
		prevMonth = 12
	}

	// 获取上个月的最大天数
	daysInPrevMonth := time.Date(prevYear, prevMonth+1, 0, 0, 0, 0, 0, time.Local).Day()

	// 如果原日期在上个月不存在（如1月31日），则使用上个月的最后一天
	if day > daysInPrevMonth {
		return time.Date(prevYear, prevMonth, daysInPrevMonth, 0, 0, 0, 0, time.Local)
	}

	return time.Date(prevYear, prevMonth, day, 0, 0, 0, 0, time.Local)
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
