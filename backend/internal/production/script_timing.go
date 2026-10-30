package production

import (
	"fmt"
	"math"
	"slices"
	"unicode"
)

// PrepareScriptShotTiming 按镜头规则计时；超过分段/模型上限时拆成连续镜头，
// 台词按原文标点优先分开，不截掉字词，也不压短说完台词所需的时间。
func PrepareScriptShotTiming(shots []ScriptShot, settings StoryboardSettings, imported bool, maxMs int64) ([]ScriptShot, error) {
	if maxMs <= 0 {
		return nil, fmt.Errorf("视频模型未声明可用的剧情段时长")
	}
	shotMs := settings.ShotMs
	if imported {
		shotMs = settings.ImportedShotMs
	}
	prepared := make([]ScriptShot, 0, len(shots))
	for _, shot := range shots {
		duration := shotMs(shot.DurationSeconds, shot.Dialogue)
		if duration <= maxMs {
			shot.DurationSeconds = float64(duration) / 1000
			prepared = append(prepared, shot)
			continue
		}
		lines := splitVideoDialogue(shot.Dialogue, settings, maxMs)
		count := max(len(lines), int((duration+maxMs-1)/maxMs))
		minimum := min(MinShotMs, maxMs)
		durations := make([]int64, count)
		var total int64
		for i := range durations {
			durations[i] = minimum
			if i < len(lines) {
				durations[i] = max(minimum, settings.SpokenMs(lines[i]))
			}
			total += durations[i]
		}
		extra := max(duration-total, 0)
		for i := range durations {
			add := min(extra, maxMs-durations[i])
			durations[i] += add
			extra -= add
			part := shot
			part.Dialogue = ""
			if i < len(lines) {
				part.Dialogue = lines[i]
			}
			if part.Dialogue == "" {
				part.Speaker = ""
				if shot.Speaker != "" && !IsNarrator(shot.Speaker) && !slices.Contains(part.Characters, shot.Speaker) {
					part.Characters = append(append([]string(nil), part.Characters...), shot.Speaker)
				}
			}
			if i > 0 {
				part.Transition = TransitionCut
			}
			part.DurationSeconds = float64(durations[i]) / 1000
			prepared = append(prepared, part)
		}
	}
	if len(prepared) > MaxScriptShots {
		return nil, fmt.Errorf("按视频模型时长拆分后共有 %d 镜，超过单集 %d 镜上限，请减少本集内容或拆成多集", len(prepared), MaxScriptShots)
	}
	return prepared, nil
}

func splitVideoDialogue(dialogue string, settings StoryboardSettings, maxMs int64) []string {
	runes := []rune(dialogue)
	capacity := max(1, int(math.Floor(float64(maxMs-settings.DialoguePauseMs())*charsPerSecond/1000)))
	var lines []string
	for len(runes) > 0 {
		end := min(len(runes), capacity)
		if end < len(runes) {
			for i := end; i > end/2; i-- {
				if unicode.IsPunct(runes[i-1]) || unicode.IsSpace(runes[i-1]) {
					end = i
					break
				}
			}
		}
		lines = append(lines, string(runes[:end]))
		runes = runes[end:]
	}
	return lines
}
