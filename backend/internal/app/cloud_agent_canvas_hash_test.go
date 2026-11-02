package app

import (
	"encoding/json"
	"errors"
	"testing"

	"gorm.io/gorm"

	"yingce/backend/internal/model"
)

func TestCloudAgentCanvasHashIgnoresNodeTimestamps(t *testing.T) {
	plain := map[string]any{"nodes": []any{layoutNode("text-1", "text", 0, 0)}, "connections": []any{}}
	if got, want := cloudAgentCanvasHash(plain), creationHash(plain); got != want {
		t.Fatalf("没有节点时间戳的画布哈希应保持不变：%s != %s", got, want)
	}

	stamped := layoutNode("text-1", "text", 0, 0)
	stamped["createdAt"] = "2026-10-21T08:00:00.000Z"
	stamped["updatedAt"] = "2026-10-21T08:00:01.000Z"
	withTimestamps := map[string]any{"nodes": []any{stamped}, "connections": []any{}}
	if cloudAgentCanvasHash(withTimestamps) != cloudAgentCanvasHash(plain) {
		t.Fatal("只补写节点 createdAt/updatedAt 不应改变画布哈希")
	}
	if _, ok := stamped["updatedAt"]; !ok {
		t.Fatal("计算哈希不应修改原文档")
	}

	edited := layoutNode("text-1", "text", 0, 0, map[string]any{"content": "用户改过"})
	edited["updatedAt"] = "2026-10-21T08:00:01.000Z"
	if cloudAgentCanvasHash(map[string]any{"nodes": []any{edited}, "connections": []any{}}) == cloudAgentCanvasHash(plain) {
		t.Fatal("时间戳以外的内容变化仍必须改变画布哈希")
	}
}

func timestampCanvasFixture(t *testing.T) (*Service, func(func(node map[string]any))) {
	t.Helper()
	s, db, _, _ := creationTestService(t)
	nodes := []any{layoutNode("text-1", "text", 0, 0), layoutNode("text-2", "text", 5, 5)}
	encode := func() string {
		raw, err := json.Marshal(map[string]any{"nodes": nodes, "connections": []any{}})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	if err := db.Create(&model.CanvasProject{ID: "layout-canvas", UserID: "user", PayloadJSON: encode()}).Error; err != nil {
		t.Fatal(err)
	}
	// 模拟浏览器自动保存：写入新内容并推进 revision。
	return s, func(change func(node map[string]any)) {
		for _, node := range nodes {
			change(node.(map[string]any))
		}
		if err := db.Model(&model.CanvasProject{}).Where("id = ? AND user_id = ?", "layout-canvas", "user").
			Updates(map[string]any{"payload_json": encode(), "revision": gorm.Expr("revision + 1")}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func currentCanvasHash(t *testing.T, s *Service) string {
	t.Helper()
	canvas, err := s.repo.CanvasProjectForUser("user", "layout-canvas")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := creationDocument(canvas.PayloadJSON)
	if err != nil {
		t.Fatal(err)
	}
	return cloudAgentCanvasHash(doc)
}

// 前端合并 Agent 写入后会给节点补写 createdAt/updatedAt 并自动保存（点「同意」前还会强制保存一次），
// 模型手上的快照不应因此被判为过期。
func TestCloudAgentArrangeNodesAcceptsSnapshotAfterTimestampAutosave(t *testing.T) {
	s, mutate := timestampCanvasFixture(t)
	snapshot := currentCanvasHash(t, s)
	mutate(func(node map[string]any) {
		node["createdAt"] = "2026-10-21T08:00:00.000Z"
		node["updatedAt"] = "2026-10-21T08:00:01.000Z"
	})
	if _, err := applyCloudAgentArrangeNodes(s.repo, "user", "layout-canvas", arrangeCall(t, map[string]any{
		"snapshotHash": snapshot,
		"mode":         "row",
	}), mustRuntimePolicy(t, s)); err != nil {
		t.Fatalf("只有节点时间戳变化时整理应成功：%v", err)
	}
}

func TestCloudAgentArrangeNodesStillRejectsContentChange(t *testing.T) {
	s, mutate := timestampCanvasFixture(t)
	snapshot := currentCanvasHash(t, s)
	mutate(func(node map[string]any) {
		node["title"] = "用户改了标题"
		node["updatedAt"] = "2026-10-21T08:00:01.000Z"
	})
	var staleErr *cloudAgentFieldArgumentError
	if _, err := applyCloudAgentArrangeNodes(s.repo, "user", "layout-canvas", arrangeCall(t, map[string]any{
		"snapshotHash": snapshot,
		"mode":         "row",
	}), mustRuntimePolicy(t, s)); !errors.As(err, &staleErr) || staleErr.Issue != "stale_snapshot" {
		t.Fatalf("节点内容变化仍应按过期快照拒绝：%v", err)
	}
}

func TestCloudAgentSnapshotsIgnoreStoryboardComposerAutosave(t *testing.T) {
	node := layoutNode("story", "script", 0, 0, map[string]any{"storyboard": map[string]any{"rows": []any{}}})
	doc := map[string]any{"nodes": []any{node}, "connections": []any{}}
	canvasHash, nodeHash := cloudAgentCanvasHash(doc), cloudAgentNodeHash(doc, "story")
	node["metadata"].(map[string]any)["storyboardComposerHeight"] = float64(104)
	node["createdAt"], node["updatedAt"] = "created", "updated"
	if cloudAgentCanvasHash(doc) != canvasHash || cloudAgentNodeHash(doc, "story") != nodeHash {
		t.Fatal("分镜输入框首次测量或时间戳自动保存使 Agent 快照失效")
	}
	if node["metadata"].(map[string]any)["storyboardComposerHeight"] != float64(104) || node["updatedAt"] != "updated" {
		t.Fatal("快照投影修改了原画布")
	}
	for _, change := range []func(map[string]any){
		func(n map[string]any) { n["height"] = float64(260) },
		func(n map[string]any) { n["position"].(map[string]any)["x"] = float64(10) },
		func(n map[string]any) {
			n["metadata"].(map[string]any)["storyboard"] = map[string]any{"rows": []any{map[string]any{"dialogue": "用户编辑"}}}
		},
		func(n map[string]any) { n["metadata"].(map[string]any)["unknownBusinessField"] = true },
	} {
		changed := cloneCreationDocument(doc)
		change(creationMaps(changed["nodes"])[0])
		if cloudAgentCanvasHash(changed) == canvasHash || cloudAgentNodeHash(changed, "story") == nodeHash {
			t.Fatal("真实几何、业务或未知字段变化必须使快照失效")
		}
	}
}

func TestCloudAgentArrangeAfterStoryboardBrowserAutosave(t *testing.T) {
	nodes := []map[string]any{layoutNode("note", "markdown", 0, 0), layoutNode("story", "script", 5, 5)}
	s, doc := layoutFixture(t, nodes)
	snapshot := cloudAgentCanvasHash(doc)
	nodes[1]["metadata"].(map[string]any)["storyboardComposerHeight"] = float64(104)
	raw, _ := json.Marshal(doc)
	stored, err := s.repo.CanvasProjectForUser("user", "layout-canvas")
	if err != nil {
		t.Fatal(err)
	}
	stored.PayloadJSON = string(raw)
	if err := s.repo.UpsertCanvasProject(stored); err != nil {
		t.Fatal(err)
	}
	if _, err := applyCloudAgentArrangeNodes(s.repo, "user", "layout-canvas", arrangeCall(t, map[string]any{"snapshotHash": snapshot, "mode": "row"}), mustRuntimePolicy(t, s)); err != nil {
		t.Fatalf("浏览器初始化分镜输入框后整理不应陷入快照重试：%v", err)
	}
}

func TestCloudAgentStoryboardReceiptProvidesSeparateCanvasSnapshot(t *testing.T) {
	s, doc := layoutFixture(t, []map[string]any{layoutNode("note", "markdown", 0, 0)})
	policy := mustRuntimePolicy(t, s)
	result, err := applyCloudAgentStoryboardMutation(s.repo, "user", "layout-canvas", cloudAgentStoryboardCall(t, "canvas_create_storyboard", "create-story", map[string]any{
		"snapshotHash": cloudAgentCanvasHash(doc), "nodeId": "story", "title": "新分镜", "rows": []map[string]any{{"durationSeconds": 5, "plotDescription": "测试镜头"}},
	}), policy)
	if err != nil {
		t.Fatal(err)
	}
	receipt := result.(map[string]any)
	nodeHash, canvasHash := stringValue(receipt["snapshotHash"]), stringValue(receipt["canvasSnapshotHash"])
	if nodeHash == "" || canvasHash == "" || canvasHash == nodeHash {
		t.Fatalf("分镜回执混淆了节点和画布版本：%#v", receipt)
	}
	state := &cloudAgentRuntime{Request: CloudAgentRequest{CanvasID: "layout-canvas"}}
	read := cloudAgentStoryboardCall(t, "canvas_read_storyboard", "read-story", map[string]any{"nodeId": "story"})
	view, err := cloudAgentReadTool(s.repo, "user", state, read)
	if err != nil {
		t.Fatal(err)
	}
	if fields := view.(map[string]any); fields["snapshotHash"] != nodeHash || fields["canvasSnapshotHash"] != canvasHash {
		t.Fatalf("分镜读取没有保留两种版本：%#v", fields)
	}
	if _, err := applyCloudAgentArrangeNodes(s.repo, "user", "layout-canvas", arrangeCall(t, map[string]any{"snapshotHash": nodeHash, "mode": "row"}), policy); err == nil {
		t.Fatal("不能用节点版本跳过整张画布并发校验")
	}
	if _, err := applyCloudAgentArrangeNodes(s.repo, "user", "layout-canvas", arrangeCall(t, map[string]any{"snapshotHash": canvasHash, "mode": "row"}), policy); err != nil {
		t.Fatalf("新建分镜后不能直接使用回执的画布版本整理：%v", err)
	}
}
