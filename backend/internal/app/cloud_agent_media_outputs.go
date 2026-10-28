package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

type cloudAgentMediaOutput struct {
	resource *model.Resource
	assetID  string
}

// Validate every output before changing the document; a bad second image must
// not turn the first image into an apparently complete generation.
func cloudAgentMediaOutputs(repo *repository.Repository, userID, kind string, task *model.Task) ([]cloudAgentMediaOutput, error) {
	id, _ := taskOutputResource(task.ResultJSON, task.Type)
	ids := []string{id}
	if kind == "image" {
		var result struct {
			Images []json.RawMessage `json:"images"`
		}
		if json.Unmarshal([]byte(task.ResultJSON), &result) == nil && len(result.Images) > 0 {
			ids = make([]string, len(result.Images))
			for index, image := range result.Images {
				ids[index], _ = taskOutputResource(string(image), task.Type)
			}
		}
	}
	outputs := make([]cloudAgentMediaOutput, 0, len(ids))
	for index, id := range ids {
		resource, err := repo.ResourceForUser(userID, id)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if err != nil || resource.Status != model.ResourceStatusReady || !strings.HasPrefix(resource.MimeType, kind+"/") {
			return nil, &cloudAgentMediaWritebackError{error: BadAuthRequest("生成结果没有可用的账号资源，未写入媒体地址"), reason: "result_resource_unavailable"}
		}
		output := cloudAgentMediaOutput{resource: resource}
		assetID := generationMediaAssetID(task.ID, index)
		if _, err := repo.AssetForUser(userID, assetID); err == nil {
			output.assetID = assetID
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}

func applyCloudAgentMediaOutput(node map[string]any, task *model.Task, output cloudAgentMediaOutput) {
	meta := node["metadata"].(map[string]any)
	resource := output.resource
	meta["content"], meta["storageKey"] = resourceFileURL(resource.ID), "resource:"+resource.ID
	meta["status"], meta["taskStatus"] = "success", string(task.Status)
	meta["naturalWidth"], meta["naturalHeight"] = resource.Width, resource.Height
	meta["bytes"], meta["mimeType"] = resource.Size, resource.MimeType
	meta["generationOutputCount"] = 1
	delete(meta, "assetId")
	if output.assetID != "" {
		meta["assetId"] = output.assetID
	}
	if resource.Width > 0 && resource.Height > 0 {
		if width := numberValue(node["width"], 0); width > 0 {
			node["height"] = width * float64(resource.Height) / float64(resource.Width)
		}
	}
	delete(meta, "errorDetails")
}

func applyCloudAgentMediaOutputs(repo *repository.Repository, userID string, doc, node map[string]any, task *model.Task) error {
	outputs, err := cloudAgentMediaOutputs(repo, userID, stringValue(node["type"]), task)
	if err != nil {
		return err
	}
	meta := node["metadata"].(map[string]any)
	// A completed replay must not restore deleted children or reset user edits.
	if numberValue(meta["generationOutputCount"], 0) == float64(len(outputs)) && meta["status"] == "success" && stringValue(meta["content"]) != "" {
		return nil
	}
	if len(outputs) == 1 {
		applyCloudAgentMediaOutput(node, task, outputs[0])
		return nil
	}

	rootID := stringValue(node["id"])
	nodes := creationMaps(doc["nodes"])
	byID := cloudAgentLayoutNodesByID(doc)
	childIDs := make([]string, len(outputs))
	// Share the browser's batch IDs and metadata so reconnecting the ordinary
	// canvas consumer adopts these nodes instead of creating duplicate results.
	for index := range outputs {
		id := fmt.Sprintf("%s:task:%s:image:%d", rootID, task.ID, index)
		childIDs[index] = id
		if existing := byID[id]; existing != nil {
			existingMeta, _ := existing["metadata"].(map[string]any)
			if stringValue(existing["type"]) != "image" || stringValue(existingMeta["taskId"]) != task.ID || stringValue(existingMeta["batchRootId"]) != rootID {
				return &cloudAgentMediaWritebackError{error: creationConflict("生成结果节点已被其他内容占用，未覆盖现有内容"), reason: "output_node_conflict"}
			}
		}
	}

	position, _ := node["position"].(map[string]any)
	x, y := numberValue(position["x"], 0), numberValue(position["y"], 0)
	width := numberValue(node["width"], 300)
	rowHeight := numberValue(node["height"], 300)
	for _, output := range outputs {
		if resource := output.resource; resource.Width > 0 && resource.Height > 0 {
			rowHeight = max(rowHeight, width*float64(resource.Height)/float64(resource.Width))
		}
	}
	for index, output := range outputs {
		id := childIDs[index]
		if byID[id] != nil {
			continue
		}
		child := cloneStringAnyMap(node)
		childMeta := cloneStringAnyMap(meta)
		child["id"], child["title"], child["metadata"] = id, fmt.Sprintf("%s · %d", stringValue(node["title"]), index+1), childMeta
		child["position"] = map[string]any{"x": x + width + 120 + float64(index%2)*(width+36), "y": y + float64(index/2)*(rowHeight+36)}
		delete(child, "parentId")
		for _, key := range []string{"isBatchRoot", "batchChildIds", "primaryImageId", "imageBatchExpanded", "mediaVersions", "currentMediaVersionId", "versionOfNodeId", "versionLabel", "versionPrimary", "agentGenerationContinuation"} {
			delete(childMeta, key)
		}
		childMeta["batchRootId"] = rootID
		applyCloudAgentMediaOutput(child, task, output)
		nodes = append(nodes, child)
		byID[id] = child
	}
	primaryID := stringValue(meta["primaryImageId"])
	primary := byID[primaryID]
	if !containsString(childIDs, primaryID) {
		primaryID, primary = childIDs[0], byID[childIDs[0]]
	}
	primaryMeta := primary["metadata"].(map[string]any)
	for _, key := range []string{"content", "storageKey", "assetId", "naturalWidth", "naturalHeight", "bytes", "mimeType"} {
		delete(meta, key)
		if value, ok := primaryMeta[key]; ok {
			meta[key] = value
		}
	}
	node["height"] = primary["height"]
	meta["status"], meta["taskStatus"], meta["generationOutputCount"] = "success", string(task.Status), len(outputs)
	meta["isBatchRoot"], meta["batchChildIds"], meta["primaryImageId"] = true, childIDs, primaryID
	if _, exists := meta["imageBatchExpanded"]; !exists {
		meta["imageBatchExpanded"] = false
	}
	delete(meta, "errorDetails")
	doc["nodes"] = nodes
	return nil
}
