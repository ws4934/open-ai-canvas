package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"gorm.io/gorm"
	"yingce/backend/internal/model"
)

func agentImageWritebackFixture(t *testing.T, resourceIDs []string) (*Service, *gorm.DB, *model.Task) {
	t.Helper()
	s, db, _ := agentMediaFixture(t)
	images := make([]map[string]any, len(resourceIDs))
	for index, id := range resourceIDs {
		images[index] = map[string]any{"dataUrl": resourceFileURL(id), "url": resourceFileURL(id), "storageKey": "resource:" + id}
		if err := db.Create(&model.Asset{ID: generationMediaAssetID("agent-image-task", index), UserID: "user", Kind: "image", PayloadJSON: `{}`}).Error; err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := json.Marshal(map[string]any{"mode": "image", "images": images})
	task := &model.Task{ID: "agent-image-task", UserID: "user", ProjectID: "agent-canvas", Type: "canvas_image", Status: model.TaskStatusSucceeded, ResultJSON: string(raw)}
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	canvas, err := s.repo.CanvasProjectForUser("user", "agent-canvas")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := creationDocument(canvas.PayloadJSON)
	if err != nil {
		t.Fatal(err)
	}
	doc["nodes"] = append(creationMaps(doc["nodes"]), map[string]any{
		"id": "image-result", "type": "image", "title": "生成图片", "position": map[string]any{"x": 800, "y": 200}, "width": 300, "height": 225,
		"metadata": map[string]any{"taskId": task.ID, "taskStatus": "running", "status": "loading", "prompt": "测试"},
	})
	saveAgentImageWritebackDocument(t, db, doc)
	return s, db, task
}

func saveAgentImageWritebackDocument(t *testing.T, db *gorm.DB, doc map[string]any) {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.CanvasProject{}).Where("id = ?", "agent-canvas").Update("payload_json", string(raw)).Error; err != nil {
		t.Fatal(err)
	}
}

func readAgentImageWritebackDocument(t *testing.T, s *Service) map[string]any {
	t.Helper()
	canvas, err := s.repo.CanvasProjectForUser("user", "agent-canvas")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := creationDocument(canvas.PayloadJSON)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestCloudAgentImageWritebackKeepsEveryOutput(t *testing.T) {
	for _, count := range []int{1, 2, 4, 32} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			ids := make([]string, count)
			for index := range ids {
				ids[index] = []string{"ref-two", "ref-one"}[index%2]
			}
			s, _, task := agentImageWritebackFixture(t, ids)
			policy, err := s.RuntimePolicy()
			if err != nil {
				t.Fatal(err)
			}
			id, err := completeCloudAgentMediaNode(s.repo, "user", "agent-canvas", "image-result", task, policy)
			if err != nil || id != "image-result" {
				t.Fatalf("writeback failed: node=%q err=%v", id, err)
			}
			doc := readAgentImageWritebackDocument(t, s)
			nodes := cloudAgentLayoutNodesByID(doc)
			meta := nodes[id]["metadata"].(map[string]any)
			if meta["storageKey"] != "resource:ref-two" || numberValue(meta["generationOutputCount"], 0) != float64(count) || meta["assetId"] != generationMediaAssetID(task.ID, 0) {
				t.Fatalf("root lost its first output or full count: %+v", meta)
			}
			if count == 1 {
				if len(nodes) != 4 || meta["isBatchRoot"] != nil {
					t.Fatal("single-image writeback changed the existing node layout")
				}
				return
			}
			children := stringIDs(meta["batchChildIds"])
			if len(children) != count || len(nodes) != 4+count || meta["isBatchRoot"] != true || meta["imageBatchExpanded"] != false {
				t.Fatalf("multi-image batch incomplete: children=%v nodes=%d", children, len(nodes))
			}
			for index, resourceID := range ids {
				childID := fmt.Sprintf("image-result:task:%s:image:%d", task.ID, index)
				child := nodes[childID]
				if children[index] != childID || child == nil {
					t.Fatalf("missing ordered image %d", index)
				}
				childMeta := child["metadata"].(map[string]any)
				if childMeta["storageKey"] != "resource:"+resourceID || childMeta["assetId"] != generationMediaAssetID(task.ID, index) || childMeta["batchRootId"] != id || childMeta["taskId"] != task.ID || childMeta["status"] != "success" {
					t.Fatalf("image %d has incorrect resource/asset binding: %+v", index, childMeta)
				}
				if numberValue(child["height"], 0) != 225 {
					t.Fatal("image aspect ratio changed")
				}
			}
		})
	}
}

func TestCloudAgentImageWritebackReplayPreservesEditsAndDeletion(t *testing.T) {
	s, db, task := agentImageWritebackFixture(t, []string{"ref-one", "ref-two"})
	policy, err := s.RuntimePolicy()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := completeCloudAgentMediaNode(s.repo, "user", "agent-canvas", "image-result", task, policy); err != nil {
		t.Fatal(err)
	}
	doc := readAgentImageWritebackDocument(t, s)
	nodes := cloudAgentLayoutNodesByID(doc)
	childID := fmt.Sprintf("image-result:task:%s:image:1", task.ID)
	nodes[childID]["title"] = "我修改的标题"
	nodes[childID]["position"] = map[string]any{"x": 2000, "y": 900}
	nodes[childID]["width"], nodes[childID]["height"] = 480, 360
	meta := nodes["image-result"]["metadata"].(map[string]any)
	meta["imageBatchExpanded"] = true
	deleteID := fmt.Sprintf("image-result:task:%s:image:0", task.ID)
	kept := []map[string]any{}
	for _, node := range creationMaps(doc["nodes"]) {
		if stringValue(node["id"]) != deleteID {
			kept = append(kept, node)
		}
	}
	doc["nodes"] = kept
	saveAgentImageWritebackDocument(t, db, doc)
	for range 2 {
		if _, err := completeCloudAgentMediaNode(s.repo, "user", "agent-canvas", "image-result", task, policy); err != nil {
			t.Fatal(err)
		}
	}
	after := cloudAgentLayoutNodesByID(readAgentImageWritebackDocument(t, s))
	if len(after) != len(kept) || after[deleteID] != nil {
		t.Fatal("replay duplicated results or restored a deleted image")
	}
	child := after[childID]
	position := child["position"].(map[string]any)
	if child["title"] != "我修改的标题" || numberValue(position["x"], 0) != 2000 || numberValue(child["width"], 0) != 480 || numberValue(child["height"], 0) != 360 || after["image-result"]["metadata"].(map[string]any)["imageBatchExpanded"] != true {
		t.Fatal("replay overwrote user edits")
	}
}

func TestCloudAgentImageWritebackValidatesEveryResourceBeforeCommit(t *testing.T) {
	for _, test := range []struct{ name, resourceID string }{
		{"missing second image", "missing"},
		{"other user's second image", "private"},
		{"unready second image", "unready"},
		{"wrong media type", "video-output"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, db, task := agentImageWritebackFixture(t, []string{"ref-one", test.resourceID})
			for _, resource := range []model.Resource{
				{ID: "unready", UserID: "user", Status: "pending", MimeType: "image/png"},
				{ID: "video-output", UserID: "user", Status: "ready", MimeType: "video/mp4"},
			} {
				if err := db.Create(&resource).Error; err != nil {
					t.Fatal(err)
				}
			}
			policy, err := s.RuntimePolicy()
			if err != nil {
				t.Fatal(err)
			}
			_, err = completeCloudAgentMediaNode(s.repo, "user", "agent-canvas", "image-result", task, policy)
			var writebackErr *cloudAgentMediaWritebackError
			if !errors.As(err, &writebackErr) || writebackErr.reason != "result_resource_unavailable" {
				t.Fatalf("invalid second output was accepted: %v", err)
			}
			nodes := cloudAgentLayoutNodesByID(readAgentImageWritebackDocument(t, s))
			meta := nodes["image-result"]["metadata"].(map[string]any)
			if len(nodes) != 4 || meta["status"] != "error" || meta["content"] != nil || meta["storageKey"] != nil || meta["isBatchRoot"] != nil {
				t.Fatal("invalid later output committed a partial image batch")
			}
		})
	}
}

func TestCloudAgentImageWritebackDoesNotOverwriteOccupiedOutputNode(t *testing.T) {
	s, db, task := agentImageWritebackFixture(t, []string{"ref-one", "ref-two"})
	doc := readAgentImageWritebackDocument(t, s)
	id := fmt.Sprintf("image-result:task:%s:image:1", task.ID)
	doc["nodes"] = append(creationMaps(doc["nodes"]), map[string]any{"id": id, "type": "text", "metadata": map[string]any{"content": "不要覆盖"}})
	saveAgentImageWritebackDocument(t, db, doc)
	before, _ := s.repo.CanvasProjectForUser("user", "agent-canvas")
	policy, err := s.RuntimePolicy()
	if err != nil {
		t.Fatal(err)
	}
	_, err = completeCloudAgentMediaNode(s.repo, "user", "agent-canvas", "image-result", task, policy)
	var writebackErr *cloudAgentMediaWritebackError
	if !errors.As(err, &writebackErr) || writebackErr.reason != "output_node_conflict" {
		t.Fatalf("occupied child node was accepted: %v", err)
	}
	after, _ := s.repo.CanvasProjectForUser("user", "agent-canvas")
	if before.PayloadJSON != after.PayloadJSON {
		t.Fatal("output conflict modified the document")
	}
}
