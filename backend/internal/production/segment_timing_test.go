package production

import (
	"strings"
	"testing"
)

func TestVideoDurationRangesAndFixedSlots(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options DurationOptions
		ms      int64
		want    int
		maximum int
	}{
		{"fractional_plan", DurationOptions{Min: 1, Max: 15, Step: 1}, 12200, 13, 15},
		{"fixed_single", DurationOptions{Values: []int{6}}, 4200, 6, 6},
		{"fixed_slots", DurationOptions{Values: []int{10, 5}}, 6100, 10, 10},
		{"long_range", DurationOptions{Min: 1, Max: 120, Step: 1}, 95200, 96, 120},
		{"step_grid", DurationOptions{Min: 1, Max: 15, Step: 4}, 12200, 13, 13},
		{"exceeds_last_step", DurationOptions{Min: 1, Max: 15, Step: 4}, 13200, 0, 13},
		{"undeclared", DurationOptions{}, 3000, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SnapVideoSeconds(tc.ms, tc.options)
			if got != tc.want || (err != nil) != (tc.want == 0) || tc.options.MaxSeconds() != tc.maximum {
				t.Fatalf("seconds=%d maximum=%d err=%v", got, tc.options.MaxSeconds(), err)
			}
		})
	}
}

func TestSegmentPromptFitsWholeVideoWithoutReversingFinalShot(t *testing.T) {
	in := SegmentPromptInput{Shots: []SegmentShot{{DurationMs: 2500}, {DurationMs: 2720}, {DurationMs: 3390}, {DurationMs: 3590}}}
	if _, err := FitSegmentVideoDuration(in, 6); err == nil {
		t.Fatal("6-second target accepted 12.2 seconds of story")
	}
	fitted, err := FitSegmentVideoDuration(in, 13)
	if err != nil {
		t.Fatal(err)
	}
	if in.Shots[3].DurationMs != 3590 || fitted.Shots[3].DurationMs != 4390 {
		t.Fatal("padding changed original shots or failed to fill the legal slot")
	}
	prompt := SegmentVideoPrompt(fitted)
	if !strings.Contains(prompt, "08.61—13.00秒") || !strings.Contains(prompt, "13秒｜") {
		t.Fatalf("invalid fitted timeline: %s", prompt)
	}
	if err := ValidateSegmentPromptTiming(prompt, 13); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSegmentPromptTiming(prompt, 13, 2500, 2720, 3390, 4390); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSegmentPromptTiming(prompt, 13, 2500, 2800, 3310, 4390); err == nil {
		t.Fatal("writer changed a shot boundary without detection")
	}
	if err := ValidateSegmentPromptTiming("没有镜头时间码的提示词", 13, 2500, 2720, 3390, 4390); err == nil {
		t.Fatal("writer dropped all shot blocks without detection")
	}
}

func TestSegmentPromptRejectsInvalidGeneratedTimelines(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prompt string
		bad    bool
	}{
		{"reverse", "【08.61—06.00秒｜镜头四：拉远】", true},
		{"zero", "2.00—2.00 秒：动作", true},
		{"past_end", "4.00—6.50秒：动作", true},
		{"negative", "-1.00—2.00秒：动作", true},
		{"gap", "【00—02秒｜镜头一】\n【03—06秒｜镜头二】", true},
		{"overlap", "【00—04秒｜镜头一】\n【03—06秒｜镜头二】", true},
		{"wrong_total", "【00—03秒｜镜头一】", true},
		{"action_outside_shot", "【00—03秒｜镜头一】\n02—04秒：推近\n【03—06秒｜镜头二】", true},
		{"nested_actions", "【00—03秒｜镜头一】\n00—01秒：起幅\n01—03秒：推近\n【03—06秒｜镜头二】", false},
		{"unitless_actions", "【00—03秒｜镜头一】\n00—01：起幅\n01—03：推近\n【03—06秒｜镜头二】", false},
		{"relative_pause_is_not_a_timecode", "【00—03秒｜镜头一】\n【03—06秒｜镜头二】\n人物停顿0.5–1秒，再抬头。", false},
		{"free_text", "角色看向门外，随后转身离开。", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateSegmentPromptTiming(tc.prompt, 6); (err != nil) != tc.bad {
				t.Fatalf("err=%v want invalid=%v", err, tc.bad)
			}
		})
	}
}

func TestFixedVideoModelSplitsStoryWithoutDroppingDialogueOrReferences(t *testing.T) {
	settings := StoryboardSettings{}
	line := "第一句话要完整保留，第二句话也不能丢，最后一句接着说。"
	source := ScriptShot{Scene: "山崖", Characters: []string{"甲", "乙"}, Props: []string{"长剑"}, Speaker: "甲", Dialogue: line, Action: "甲走向山崖", DurationSeconds: 16.2, SourceQuote: line, Transition: TransitionBlack}
	shots, err := PrepareScriptShotTiming([]ScriptShot{source}, settings, true, 5000)
	if err != nil {
		t.Fatal(err)
	}
	var dialogue strings.Builder
	var total int64
	for i, shot := range shots {
		ms := int64(shot.DurationSeconds * 1000)
		if ms <= 0 || ms > 5000 || ms < settings.SpokenMs(shot.Dialogue) {
			t.Fatalf("shot %d cannot fit speech in fixed model: %+v", i, shot)
		}
		if shot.Scene != source.Scene || len(shot.Characters) != 2 || len(shot.Props) != 1 || shot.SourceQuote != source.SourceQuote {
			t.Fatal("split lost the original references or source")
		}
		if i > 0 && shot.Transition != TransitionCut {
			t.Fatal("split repeated the original scene transition")
		}
		dialogue.WriteString(shot.Dialogue)
		total += ms
	}
	if dialogue.String() != line || total < 16200 || len(shots) < 4 {
		t.Fatalf("split lost content: dialogue=%q total=%d count=%d", dialogue.String(), total, len(shots))
	}
	scenes, durations := make([]string, len(shots)), make([]int64, len(shots))
	for i, shot := range shots {
		scenes[i], durations[i] = shot.Scene, int64(shot.DurationSeconds*1000)
	}
	for _, segment := range SegmentShotsForVideo(scenes, durations, settings.SegmentLimits(5)) {
		var planned int64
		for _, ms := range durations[segment.Start:segment.End] {
			planned += ms
		}
		if seconds, err := SnapVideoSeconds(planned, DurationOptions{Values: []int{5}}); err != nil || seconds != 5 {
			t.Fatalf("split segment cannot use fixed slot: planned=%d seconds=%d err=%v", planned, seconds, err)
		}
	}
}
