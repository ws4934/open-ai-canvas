package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

func TestStorageCapacityGrantsGraceAndAuditRollback(t *testing.T) {
	s, db := newMediaRecoveryTestService(t)
	admin := &model.User{ID: "admin", Username: "admin", Role: model.UserRoleAdmin}
	user := &model.User{ID: "user", Username: "user", Role: model.UserRoleUser}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	base := gigabytes(defaultRuntimePolicy().Resource.StoredFileGB)
	resource := model.Resource{ID: "resource", UserID: user.ID, Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "large", Size: base + 1}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	usage, err := s.computeAccountStorageStatus(user.ID, now, true)
	if err != nil || !usage.OverLimit || usage.ReadOnly {
		t.Fatalf("grace: %+v %v", usage, err)
	}
	usage, err = s.computeAccountStorageStatus(user.ID, now.Add(7*24*time.Hour), true)
	if err != nil || !usage.ReadOnly {
		t.Fatalf("read-only: %+v %v", usage, err)
	}
	if err := s.requireStoredFileCapacityForTask(user.ID, "canvas_image", defaultRuntimePolicy()); err == nil {
		t.Fatal("overfull account admitted a media task")
	}
	grant, err := s.AdminGrantUserStorage(admin, user.ID, AdminStorageGrantRequest{Bytes: 1 << 30, DurationDays: 0})
	if err != nil {
		t.Fatal(err)
	}
	usage, err = s.AccountFileStorageUsage(user.ID)
	if err != nil || usage.ReadOnly || usage.OverLimit || usage.TotalBytes != base+(1<<30) {
		t.Fatalf("grant did not restore capacity: %+v %v", usage, err)
	}
	if err := s.requireStoredFileCapacityForTask(user.ID, "canvas_image", defaultRuntimePolicy()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdminGrantUserStorage(user, user.ID, AdminStorageGrantRequest{Bytes: 1 << 30}); err == nil {
		t.Fatal("non-admin grant accepted")
	}
	if err := db.Migrator().DropTable(&model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdminGrantUserStorage(admin, user.ID, AdminStorageGrantRequest{Bytes: 1 << 30}); err == nil {
		t.Fatal("grant without audit succeeded")
	}
	if _, err := s.AdminRevokeUserStorageGrant(admin, grant.ID, RevokeStorageGrantRequest{Reason: "test"}); err == nil {
		t.Fatal("revocation without audit succeeded")
	}
	var grants []model.UserStorageGrant
	if err := db.Find(&grants).Error; err != nil || len(grants) != 1 || grants[0].RevokedAt != nil {
		t.Fatalf("failed audit committed capacity mutation: %+v %v", grants, err)
	}
	if err := db.Model(&grants[0]).Update("expires_at", now.Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	usage, err = s.AccountFileStorageUsage(user.ID)
	if err != nil || usage.PurchasedBytes != 0 || !usage.OverLimit {
		t.Fatalf("expired grant counted: %+v %v", usage, err)
	}
}

func TestDramaResourcesRemainReferencedUntilProjectReferencesRemoved(t *testing.T) {
	s, db := newMediaRecoveryTestService(t)
	resource := model.Resource{ID: "drama-resource", UserID: "owner", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "movie.mp4", Kind: "video"}
	for _, row := range []any{
		&model.Project{ID: "drama-project", UserID: "owner", Name: "Drama"}, &resource,
		&model.DramaCast{ID: "cast", ProjectID: "drama-project", ImageResourceID: resource.ID},
		&model.DramaEpisode{ID: "episode", ProjectID: "drama-project", Number: 1, FinalResourceID: resource.ID},
		&model.DramaSegment{ID: "segment", ProjectID: "drama-project", EpisodeID: "episode", VideoResourceID: resource.ID, TailFrameResourceID: resource.ID},
		&model.DramaTake{ID: "take", ProjectID: "drama-project", OwnerID: "segment", Kind: "video", Version: 1, ResourceID: resource.ID},
		&model.Asset{ID: "asset", UserID: "owner", Kind: "video", PayloadJSON: `{"url":"/api/resources/drama-resource/file"}`},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	refs, err := s.repo.DramaResourceReferences("owner", []string{resource.ID})
	if err != nil || len(refs) != 5 {
		t.Fatalf("drama references: %+v %v", refs, err)
	}
	refs, err = s.repo.DramaResourceReferences("other", []string{resource.ID})
	if err != nil || len(refs) != 0 {
		t.Fatalf("cross-user references: %+v %v", refs, err)
	}
	if err := s.repo.DeleteDetachedResources([]model.Resource{resource}, nil); !errors.Is(err, repository.ErrResourceCleanupStillReferenced) {
		t.Fatalf("cleanup removed drama media: %v", err)
	}
	if err := s.deleteUserAssetWithResources("owner", "asset", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.repo.Resource(resource.ID); err != nil {
		t.Fatalf("asset purge removed drama media: %v", err)
	}
}

func TestGeneratedMediaFallbackAndOffloadPreserveDelivery(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	s, db := newMediaRecoveryTestService(t)
	var healthy atomic.Bool
	var putCalls atomic.Int32
	image := mediaTestPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected %s", r.Method)
			w.WriteHeader(400)
			return
		}
		putCalls.Add(1)
		if !healthy.Load() {
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`<Error><Code>ServiceUnavailable</Code></Error>`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !bytes.Equal(body, image) {
			t.Error("offloaded bytes changed")
		}
		w.Header().Set("ETag", "test-etag")
		w.WriteHeader(200)
	}))
	defer server.Close()
	setting, _ := json.Marshal(ossSettingValue{Enabled: true, Provider: "aliyun", Endpoint: server.URL, Bucket: "test", AccessKeyID: "test-key", AccessKeySecret: "test-secret"})
	if err := s.repo.SaveSystemSetting(&model.SystemSetting{Key: ossSettingKey, ValueJSON: string(setting)}); err != nil {
		t.Fatal(err)
	}
	task := seedMediaTask(t, db, providerConfig{})
	staged := filepath.Join(t.TempDir(), "generated.png")
	if err := os.WriteFile(staged, image, 0o600); err != nil {
		t.Fatal(err)
	}
	resource, cause, err := s.storeTaskMediaFile(task, 0, staged, "image/png", "image", nil)
	if err != nil || cause == nil || resource == nil || resource.Provider != "local" || resource.Status != model.ResourceStatusReady {
		t.Fatalf("fallback: %+v %v %v", resource, cause, err)
	}
	localKey := resource.ObjectKey
	delivery, err := s.PrepareResourceDelivery(task.UserID, resource.ID, ResourceAccessOptions{}, "")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(delivery.Stream.Body)
	_ = delivery.Stream.Body.Close()
	if !bytes.Equal(body, image) {
		t.Fatal("local fallback not delivered")
	}
	var job model.ResourceOffloadJob
	if err := db.First(&job, "resource_id = ?", resource.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&job).Update("next_attempt_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	healthy.Store(true)
	s.drainResourceOffloadJobs(context.Background(), 1)
	stored, err := s.repo.Resource(resource.ID)
	if err != nil || stored.Provider != "aliyun" || stored.ID != resource.ID || stored.ObjectKey != job.TargetObjectKey {
		t.Fatalf("offload switch: %+v %v", stored, err)
	}
	if putCalls.Load() != 2 {
		t.Fatalf("unexpected uploads %d", putCalls.Load())
	}
	if _, err := os.Stat(filepath.Join(s.dataDir, "resources", localKey)); err != nil {
		t.Fatal("local file removed before retention ended")
	}
	if err := db.Model(&job).Update("local_delete_after", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	s.cleanupOffloadedLocalFiles()
	if _, err := os.Stat(filepath.Join(s.dataDir, "resources", localKey)); !os.IsNotExist(err) {
		t.Fatalf("retained local object not cleaned: %v", err)
	}
}

func TestOffloadWaitsForPendingResourceCommit(t *testing.T) {
	s, db := newMediaRecoveryTestService(t)
	if err := os.MkdirAll(filepath.Join(s.dataDir, "resources"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.dataDir, "resources", "pending.png")
	if err := os.WriteFile(path, []byte("pending"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Resource{ID: "pending", UserID: "user", Status: model.ResourceStatusPending, Provider: "aliyun", ObjectKey: "target.png"}).Error; err != nil {
		t.Fatal(err)
	}
	job := model.ResourceOffloadJob{ID: "job", UserID: "user", ResourceID: "pending", LocalObjectKey: "pending.png", Status: model.ResourceOffloadStatusPending, NextAttemptAt: time.Now().Add(-time.Minute)}
	if err := s.repo.UpsertResourceOffloadJob(&job); err != nil {
		t.Fatal(err)
	}
	s.drainResourceOffloadJobs(context.Background(), 1)
	if err := db.First(&job, "id = ?", job.ID).Error; err != nil || job.Status != model.ResourceOffloadStatusPending || job.Attempts != 1 {
		t.Fatalf("pending writer was cancelled: %+v %v", job, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("pending local file removed: %v", err)
	}
}
