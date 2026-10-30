package production

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
)

// FitSegmentVideoDuration 只延长末镜补足合法档位，不能靠负时长压缩已有分镜。
func FitSegmentVideoDuration(input SegmentPromptInput, seconds int) (SegmentPromptInput, error) {
	var total int64
	for _, shot := range input.Shots {
		if shot.DurationMs <= 0 {
			return input, fmt.Errorf("分镜包含无效时长，请调整后重新撰写提示词")
		}
		total += shot.DurationMs
	}
	target := int64(seconds) * 1000
	if len(input.Shots) == 0 || target < total || target <= 0 {
		return input, fmt.Errorf("生成时长 %d 秒不足以容纳 %.1f 秒分镜，请拆分剧情段或重新改编", seconds, float64(total)/1000)
	}
	input.Shots = append([]SegmentShot(nil), input.Shots...)
	input.Shots[len(input.Shots)-1].DurationMs += target - total
	return input, nil
}

// 行首/块首才是绝对时间码；“停顿0.5–1秒”这类正文时长描述不能当成段内起止。
var segmentTimeRange = regexp.MustCompile(`(?m)(?:^|【|\[)\s*(?:[-*•]\s+)?(-?[0-9]+(?:\.[0-9]+)?)\s*(?:—|–|-|~|～|至)\s*(-?[0-9]+(?:\.[0-9]+)?)\s*(?:秒|[sS])?\s*(?:[：:｜|】\]]|$)`)
var segmentShotTimeRange = regexp.MustCompile(`【\s*([0-9]+(?:\.[0-9]+)?)\s*(?:—|–|-|~|～|至)\s*([0-9]+(?:\.[0-9]+)?)\s*秒\s*[｜|]\s*镜头[^】]*】`)

// ValidateSegmentPromptTiming 检查正文明确写出的时间段。镜头内可嵌套动作时间片，
// 因此只要求镜头标题连续；全部时间片都必须正向且落在生成时长内。
func ValidateSegmentPromptTiming(prompt string, seconds int, shotDurations ...int64) error {
	limit := float64(seconds)
	for _, match := range segmentTimeRange.FindAllStringSubmatch(prompt, -1) {
		start, _ := strconv.ParseFloat(match[1], 64)
		end, _ := strconv.ParseFloat(match[2], 64)
		if start < 0 || start >= end || (limit > 0 && end > limit+0.01) {
			return fmt.Errorf("视频提示词时间码 %s 无效：起点必须早于终点，且不能超过生成时长 %d 秒；请重新撰写或修改后保存", match[0], seconds)
		}
	}
	headings := segmentShotTimeRange.FindAllStringSubmatchIndex(prompt, -1)
	if len(shotDurations) > 0 && len(headings) != len(shotDurations) {
		return fmt.Errorf("视频提示词需要保留 %d 个镜头及其时间码，当前只有 %d 个镜头；请重新撰写", len(shotDurations), len(headings))
	}
	cursor := 0.0
	var expectedEnd int64
	for i, heading := range headings {
		start, _ := strconv.ParseFloat(prompt[heading[2]:heading[3]], 64)
		end, _ := strconv.ParseFloat(prompt[heading[4]:heading[5]], 64)
		if math.Abs(start-cursor) > 0.01 {
			return fmt.Errorf("视频提示词镜头时间码不连续：%.2f 秒后应接下一镜，不能跳到 %.2f 秒；请重新撰写或修改后保存", cursor, start)
		}
		if len(shotDurations) > 0 {
			expectedEnd += shotDurations[i]
			if math.Abs(end-float64(expectedEnd)/1000) > 0.01 {
				return fmt.Errorf("视频提示词第 %d 镜应结束于 %.2f 秒，模型写成 %.2f 秒；请按分镜时间码重新撰写", i+1, float64(expectedEnd)/1000, end)
			}
		}
		bodyEnd := len(prompt)
		if i+1 < len(headings) {
			bodyEnd = headings[i+1][0]
		}
		for _, action := range segmentTimeRange.FindAllStringSubmatch(prompt[heading[1]:bodyEnd], -1) {
			actionStart, _ := strconv.ParseFloat(action[1], 64)
			actionEnd, _ := strconv.ParseFloat(action[2], 64)
			if actionStart < start-0.01 || actionEnd > end+0.01 {
				return fmt.Errorf("视频提示词动作时间码 %s 超出本镜头 %.2f—%.2f 秒的范围；请重新撰写或修改后保存", action[0], start, end)
			}
		}
		cursor = end
	}
	if len(headings) > 0 && limit > 0 && math.Abs(cursor-limit) > 0.01 {
		return fmt.Errorf("视频提示词镜头结束于 %.2f 秒，与生成时长 %d 秒不一致；请重新撰写或修改后保存", cursor, seconds)
	}
	return nil
}
