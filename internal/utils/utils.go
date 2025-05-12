package utils

import (
    "fmt"
    "math"
    "time"
)

func CalculateLastMonthExpiry(expiryTime time.Time, now time.Time) time.Time {
    dayOfMonth := expiryTime.Day()
    year, month, _ := expiryTime.Date()
    for {
        // 推到上一个月
        if month == 1 {
            year--
            month = 12
        } else {
            month--
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

func CalculateDaysDifference(now, lastMonthExpiry time.Time) int {
    daysDiff := int(math.Floor(now.Sub(lastMonthExpiry).Hours() / 24))
    if daysDiff < 0 {
        daysDiff = -daysDiff
    }
    return daysDiff
}


func CalculateTimeLeft(timeLeft time.Duration) (int, int, int) {
    yearsLeft := int(timeLeft.Hours() / (24 * 365))
    monthsLeft := int((timeLeft.Hours() / (24 * 30)) - (float64(yearsLeft) * 12))
    daysLeft := int(timeLeft.Hours()/24) - (yearsLeft * 365) - (monthsLeft * 30)
    return yearsLeft, monthsLeft, daysLeft
}


func FormatDuration(d time.Duration) string {
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
