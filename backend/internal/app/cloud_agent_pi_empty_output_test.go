package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"yingce/backend/internal/model"
)

func TestCloudAgentPiEmptyOutputCorrectsAndStopsWithinRetryBudget(t *testing.T) {
	for _, success := range []bool{true, false} {
		name := "exhausted"
		if success {
			name = "recovered"
		}
		t.Run(name, func(t *testing.T) {
			s, db, runID := piOutputLimitFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := s.cloudAgentPiModel(ctx, "user", runID, map[string]json.RawMessage{
					"messages": json.RawMessage(`[{"role":"user","content":"继续制作"}]`), "thinkingLevel": json.RawMessage(`"high"`),
				})
				done <- err
			}()
			previous := ""
			for attempt := 0; attempt <= cloudAgentModelStepRetries; attempt++ {
				var task model.Task
				for task.ID == "" {
					_, state := agentInterjectionState(t, s, runID)
					if state.ActiveTaskID != "" && state.ActiveTaskID != previous {
						if err := db.First(&task, "id = ?", state.ActiveTaskID).Error; err != nil {
							t.Fatal(err)
						}
						break
					}
					select {
					case err := <-done:
						t.Fatalf("重试提前结束：%v", err)
					case <-ctx.Done():
						t.Fatal("没有调度纠正任务")
					case <-time.After(10 * time.Millisecond):
					}
				}
				if attempt > 0 && !strings.Contains(task.InputJSON, "empty_output") {
					t.Fatal("空响应被原样重发，没有纠正上下文")
				}
				var input struct {
					TextOptions struct {
						Thinking bool `json:"thinking"`
					} `json:"textOptions"`
				}
				if err := json.Unmarshal([]byte(task.InputJSON), &input); err != nil {
					t.Fatal(err)
				}
				if input.TextOptions.Thinking != (attempt < cloudAgentModelStepRetries) {
					t.Fatalf("第 %d 步没有采用升级后的思考设置：%s", attempt, task.InputJSON)
				}
				updates := map[string]any{"status": model.TaskStatusFailed, "error": "声明式 Agent 接口没有返回内容"}
				if success && attempt == cloudAgentModelStepRetries {
					updates = map[string]any{"status": model.TaskStatusSucceeded, "result_json": `{"text":"已继续制作","toolCalls":[]}`}
				}
				if err := db.Model(&model.Task{}).Where("id = ?", task.ID).Updates(updates).Error; err != nil {
					t.Fatal(err)
				}
				previous = task.ID
			}
			select {
			case err := <-done:
				if (err == nil) != success {
					t.Fatalf("空响应重试结果不符：%v", err)
				}
			case <-ctx.Done():
				t.Fatal("空响应重试耗尽后没有结束")
			}
			_, state := agentInterjectionState(t, s, runID)
			if state.ActiveTaskID != "" || len(state.Calls) != 0 || state.EmptyOutputNudged != cloudAgentMaxEmptyOutputNudges || state.EmptyOutputEscalated != 1 || !state.BoostStepOutputBudget {
				t.Fatalf("空响应恢复没有释放任务或保存升级设置：%+v", state)
			}
			for _, event := range state.Events {
				if event.Type == "canvas_updated" || event.Type == "generation_task_created" {
					t.Fatal("空响应重试执行了画布或媒体工具")
				}
			}
		})
	}
}
