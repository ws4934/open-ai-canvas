package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
	"yingce/backend/internal/model"
	"yingce/backend/internal/protocol"
)

const (
	agentPromptCacheCreateTimeout = 3 * time.Second
	agentPromptCacheDeleteTimeout = 2 * time.Second
	agentPromptCacheLockTimeout   = 750 * time.Millisecond
	agentPromptCacheLeaseTTL      = 10 * time.Second
)

type agentPromptCacheKeyLock struct {
	token chan struct{}
	refs  int
}

// Cache lifetime and credential/user isolation belong to the host. The plugin
// owns prefix fields, requests, minimum context size and resource validation.
func prepareAgentPromptCache(ctx context.Context, input canvasGenerationInput, spec protocol.RequestSpec, rebuild bool) (protocol.RequestSpec, bool, error) {
	base, err := cloneProtocolRequestSpec(spec)
	if err != nil {
		return spec, false, err
	}
	cacheSpec := base.PromptCache
	if cacheSpec == nil {
		return base, false, nil
	}
	stable := cacheSpec.Prefix(protocolBodyObject(base.Body))
	raw, err := json.Marshal(stable)
	if err != nil {
		return base, false, err
	}
	if len(stable) == 0 || (len(raw)+3)/4 < cacheSpec.MinimumTokens {
		return base, false, nil
	}
	metadata, ok := ctx.Value(providerAnalyticsKey{}).(providerAnalyticsContext)
	if !ok || metadata.Service == nil || metadata.Service.repo == nil || metadata.UserID == "" {
		return base, false, nil
	}
	s := metadata.Service
	key, credential, err := agentPromptCacheKey(input, metadata.UserID, stable, cacheSpec)
	if err != nil {
		return base, false, err
	}
	if !rebuild {
		cached, err := s.repo.GeminiCacheByKey(metadata.UserID, key)
		if err == nil && agentPromptCacheUsable(cached, cacheSpec) {
			return cacheSpec.Apply(base, cached.ResourceName), true, nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return base, false, err
		}
	}
	lockCtx, cancel := context.WithTimeout(ctx, agentPromptCacheLockTimeout)
	defer cancel()
	release, acquired := s.acquireAgentPromptCacheLock(lockCtx, key)
	if !acquired {
		return base, false, nil
	}
	defer release()
	cached, lookupErr := s.repo.GeminiCacheByKey(metadata.UserID, key)
	if lookupErr == nil && agentPromptCacheUsable(cached, cacheSpec) {
		return cacheSpec.Apply(base, cached.ResourceName), true, nil
	}
	if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return base, false, lookupErr
	}
	if lookupErr == nil {
		deleteAgentPromptCache(ctx, input.Config, cacheSpec, cached.ResourceName)
		deleted, err := s.repo.DeleteGeminiCacheIfResourceMatches(metadata.UserID, key, cached.ResourceName)
		if err != nil {
			return base, false, err
		}
		if !deleted {
			latest, err := s.repo.GeminiCacheByKey(metadata.UserID, key)
			if err == nil && agentPromptCacheUsable(latest, cacheSpec) {
				return cacheSpec.Apply(base, latest.ResourceName), true, nil
			}
		}
	}
	request, err := cacheSpec.BuildCreate(stable)
	if err != nil {
		return base, false, err
	}
	createCtx, createCancel := context.WithTimeout(withProviderRequestKind(ctx, "cache-create"), agentPromptCacheCreateTimeout)
	defer createCancel()
	response, err := executeProtocolRequest(createCtx, input.Config, request)
	if err != nil {
		return base, false, err
	}
	resource, expiry, err := cacheSpec.ParseCreate(response)
	if err != nil {
		return base, false, err
	}
	expires, err := time.Parse(time.RFC3339, expiry)
	if err != nil {
		deleteAgentPromptCache(ctx, input.Config, cacheSpec, resource)
		return base, false, fmt.Errorf("Prompt Cache 过期时间无效：%w", err)
	}
	record := &model.CloudAgentGeminiCache{ID: "pcache_" + key, UserID: metadata.UserID, CacheKey: key, BaseURL: strings.TrimRight(input.Config.BaseURL, "/"), Model: cacheSpec.Model, CredentialHash: credential, ResourceName: resource, ExpireTime: expires}
	if err := s.repo.UpsertGeminiCache(record); err != nil {
		deleteAgentPromptCache(ctx, input.Config, cacheSpec, resource)
		return base, false, err
	}
	return cacheSpec.Apply(base, resource), true, nil
}

func agentPromptCacheUsable(record *model.CloudAgentGeminiCache, spec *protocol.PromptCacheSpec) bool {
	return record != nil && spec != nil && spec.ValidateResource(record.ResourceName) == nil && record.ExpireTime.After(time.Now().Add(30*time.Second))
}

func agentPromptCacheKey(input canvasGenerationInput, userID string, stable map[string]any, spec *protocol.PromptCacheSpec) (string, string, error) {
	credential := agentPromptCacheHash([]byte(input.Config.APIKey))
	headers := make([]OutboundHeader, 0, len(input.Config.Headers))
	for _, h := range input.Config.Headers {
		headers = append(headers, OutboundHeader{Name: strings.ToLower(strings.TrimSpace(h.Name)), Value: strings.TrimSpace(h.Value)})
	}
	sort.Slice(headers, func(i, j int) bool {
		if headers[i].Name == headers[j].Name {
			return headers[i].Value < headers[j].Value
		}
		return headers[i].Name < headers[j].Name
	})
	raw, err := json.Marshal(map[string]any{"version": "v3", "user": userID, "credential": credential, "channel": input.Config.ChannelID, "protocol": input.Config.InterfaceType, "baseURL": strings.TrimRight(input.Config.BaseURL, "/"), "model": spec.Model, "headers": headers, "prefix": stable, "contract": spec.Definition})
	if err != nil {
		return "", "", err
	}
	return agentPromptCacheHash(raw), credential, nil
}
func agentPromptCacheHash(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func invalidateAgentPromptCache(ctx context.Context, input canvasGenerationInput, spec protocol.RequestSpec, expected string) error {
	metadata, ok := ctx.Value(providerAnalyticsKey{}).(providerAnalyticsContext)
	if !ok || metadata.Service == nil || metadata.Service.repo == nil || metadata.UserID == "" || spec.PromptCache == nil {
		return nil
	}
	c := spec.PromptCache
	key, _, err := agentPromptCacheKey(input, metadata.UserID, c.Prefix(protocolBodyObject(spec.Body)), c)
	if err != nil {
		return err
	}
	lockCtx, cancel := context.WithTimeout(ctx, agentPromptCacheLockTimeout)
	defer cancel()
	release, ok := metadata.Service.acquireAgentPromptCacheLock(lockCtx, key)
	if !ok {
		return nil
	}
	defer release()
	cached, err := metadata.Service.repo.GeminiCacheByKey(metadata.UserID, key)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if expected != "" && cached.ResourceName != expected {
		return nil
	}
	deleteAgentPromptCache(ctx, input.Config, c, cached.ResourceName)
	_, err = metadata.Service.repo.DeleteGeminiCacheIfResourceMatches(metadata.UserID, key, cached.ResourceName)
	return err
}

func deleteAgentPromptCache(ctx context.Context, config providerConfig, spec *protocol.PromptCacheSpec, resource string) {
	request, err := spec.BuildDelete(resource)
	if err != nil {
		return
	}
	timeout, cancel := context.WithTimeout(withProviderRequestKind(ctx, "cache-delete"), agentPromptCacheDeleteTimeout)
	defer cancel()
	if _, err := executeProtocolRequest(timeout, config, request); err != nil {
		log.Printf("agent prompt cache cleanup failed: model=%s reason=%s", safeProviderModel(config.Model), safeProviderLogError(err))
	}
}

func cloneProtocolRequestSpec(spec protocol.RequestSpec) (protocol.RequestSpec, error) {
	raw, err := json.Marshal(spec)
	if err != nil {
		return spec, fmt.Errorf("复制协议请求失败：%w", err)
	}
	var cloned protocol.RequestSpec
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return spec, err
	}
	return cloned, nil
}

func (s *Service) acquireAgentPromptCacheLock(ctx context.Context, cacheKey string) (func(), bool) {
	if s == nil || strings.TrimSpace(cacheKey) == "" {
		return nil, false
	}
	s.promptCacheLockMu.Lock()
	if s.promptCacheLocks == nil {
		s.promptCacheLocks = make(map[string]*agentPromptCacheKeyLock)
	}
	lock := s.promptCacheLocks[cacheKey]
	if lock == nil {
		lock = &agentPromptCacheKeyLock{token: make(chan struct{}, 1)}
		lock.token <- struct{}{}
		s.promptCacheLocks[cacheKey] = lock
	}
	lock.refs++
	s.promptCacheLockMu.Unlock()

	select {
	case <-lock.token:
		var once sync.Once
		localRelease := func() {
			once.Do(func() {
				lock.token <- struct{}{}
				s.releaseAgentPromptCacheLock(cacheKey, lock)
			})
		}
		if s.coordinator == nil {
			return localRelease, true
		}
		distributedRelease, err := s.coordinator.AcquireWithWait(ctx, "agent-prompt-cache:"+cacheKey, 1, agentPromptCacheLeaseTTL)
		if err != nil {
			localRelease()
			return nil, false
		}
		return func() {
			distributedRelease()
			localRelease()
		}, true
	case <-ctx.Done():
		s.releaseAgentPromptCacheLock(cacheKey, lock)
		return nil, false
	}
}

func (s *Service) releaseAgentPromptCacheLock(cacheKey string, lock *agentPromptCacheKeyLock) {
	if s == nil || lock == nil {
		return
	}
	s.promptCacheLockMu.Lock()
	defer s.promptCacheLockMu.Unlock()
	lock.refs--
	if lock.refs <= 0 && s.promptCacheLocks[cacheKey] == lock {
		delete(s.promptCacheLocks, cacheKey)
	}
}

func safeProviderModel(modelName string) string {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return "unknown"
	}
	return truncateRunes(modelName, 120)
}
