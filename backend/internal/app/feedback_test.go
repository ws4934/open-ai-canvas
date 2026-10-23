package app

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yingce/backend/internal/database"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestFeedbackService(t *testing.T) (*Service, *model.User, *model.User, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "feedback.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.MigrateSchema(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := repository.New(db)
	svc := &Service{repo: repo, dataDir: t.TempDir()}

	admin := &model.User{ID: "admin_1", Username: "admin", Role: "admin", Status: "active", CreatedAt: time.Now()}
	user := &model.User{ID: "user_1", Username: "testuser", Email: "test@example.com", Role: "user", Status: "active", CreatedAt: time.Now()}

	if err := db.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}

	return svc, admin, user, db
}

func TestFeedbackSubmitAndList(t *testing.T) {
	svc, admin, user, db := newTestFeedbackService(t)
	if err := db.Create(&model.Resource{ID: "res_1", UserID: user.ID, Kind: "image", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "screen.png", MimeType: "image/png", Size: 1024}).Error; err != nil {
		t.Fatal(err)
	}

	// 1. Submit feedback as user
	req := SubmitFeedbackRequest{
		Type:        model.FeedbackTypeBug,
		Title:       "视频下载按钮未响应",
		Content:     "在生成视频后，点击卡片右上角的下载按钮无反应。",
		Contact:     "test@example.com",
		Attachments: `[{"id":"res_1","name":"screen.png","type":"image","url":"/api/resources/res_1/file","size":1024}]`,
		Environment: `{"browser":"Chrome 120","os":"macOS"}`,
	}

	fb, err := svc.SubmitFeedback(user, req)
	if err != nil {
		t.Fatalf("SubmitFeedback failed: %v", err)
	}
	if fb.ID == "" || fb.Status != model.FeedbackStatusPending {
		t.Fatalf("unexpected feedback created: %+v", fb)
	}

	// 2. User view their own feedbacks
	myList, err := svc.MyFeedbacks(user)
	if err != nil {
		t.Fatalf("MyFeedbacks failed: %v", err)
	}
	if len(myList) != 1 || myList[0].Title != req.Title {
		t.Fatalf("expected 1 feedback, got %d", len(myList))
	}

	// 3. Admin view feedbacks
	adminList, err := svc.AdminListFeedbacks(admin, repository.FeedbackFilter{Status: "pending"})
	if err != nil {
		t.Fatalf("AdminListFeedbacks failed: %v", err)
	}
	if adminList.Total != 1 || len(adminList.List) != 1 {
		t.Fatalf("admin expected 1 feedback, got total=%d", adminList.Total)
	}

	// 4. Admin reply and resolve
	updated, err := svc.AdminUpdateFeedback(admin, fb.ID, AdminUpdateFeedbackRequest{
		Status:     model.FeedbackStatusResolved,
		AdminReply: "已修复该问题，感谢您的反馈！",
	})
	if err != nil {
		t.Fatalf("AdminUpdateFeedback failed: %v", err)
	}
	if updated.Status != model.FeedbackStatusResolved || updated.AdminReply == "" || updated.RepliedAt == nil {
		t.Fatalf("AdminUpdateFeedback not updated properly: %+v", updated)
	}

	// 5. User checks inbox again
	myList2, err := svc.MyFeedbacks(user)
	if err != nil {
		t.Fatalf("MyFeedbacks 2 failed: %v", err)
	}
	if myList2[0].Status != model.FeedbackStatusResolved || myList2[0].AdminReply != "已修复该问题，感谢您的反馈！" {
		t.Fatalf("user inbox did not reflect admin reply: %+v", myList2[0])
	}

	// 6. Admin archives feedback
	archived, err := svc.AdminArchiveFeedback(admin, fb.ID, true)
	if err != nil {
		t.Fatalf("AdminArchiveFeedback failed: %v", err)
	}
	if !archived.Archived || archived.ArchivedAt == nil {
		t.Fatalf("expected feedback to be archived: %+v", archived)
	}

	// 7. Verify admin filter by archived
	archivedFilter := true
	adminArchivedList, err := svc.AdminListFeedbacks(admin, repository.FeedbackFilter{Archived: &archivedFilter})
	if err != nil || adminArchivedList.Total != 1 {
		t.Fatalf("expected 1 archived feedback, got %d", adminArchivedList.Total)
	}

	notArchivedFilter := false
	adminActiveList, err := svc.AdminListFeedbacks(admin, repository.FeedbackFilter{Archived: &notArchivedFilter})
	if err != nil || adminActiveList.Total != 0 {
		t.Fatalf("expected 0 active feedbacks, got %d", adminActiveList.Total)
	}
}

func TestFeedbackAttachmentsRequireOwnedReadyResources(t *testing.T) {
	svc, admin, user, db := newTestFeedbackService(t)
	for _, resource := range []model.Resource{
		{ID: "owned", UserID: user.ID, Kind: "image", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "screens/photo.png", MimeType: "image/png", Size: 42},
		{ID: "foreign", UserID: admin.ID, Kind: "image", Status: model.ResourceStatusReady},
		{ID: "pending", UserID: user.ID, Kind: "image", Status: model.ResourceStatusPending},
		{ID: "audio", UserID: user.ID, Kind: "audio", Status: model.ResourceStatusReady},
	} {
		if err := db.Create(&resource).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{`{`, `null`, `[{}]`, `[{"id":"foreign"}]`, `[{"id":"missing"}]`, `[{"id":"pending"}]`, `[{"id":"audio"}]`, `[{"id":"owned"},{"id":"owned"}]`} {
		if _, err := svc.SubmitFeedback(user, SubmitFeedbackRequest{Title: "test", Content: "test", Attachments: raw}); err == nil {
			t.Errorf("accepted invalid attachments %s", raw)
		}
	}
	feedback, err := svc.SubmitFeedback(user, SubmitFeedbackRequest{Title: "test", Content: "test", Attachments: `[{"id":"owned","url":"https://untrusted.invalid/track","name":"forged","type":"video","size":999,"mimeType":"text/html"}]`})
	if err != nil {
		t.Fatal(err)
	}
	var attachments []model.FeedbackAttachment
	if err := json.Unmarshal([]byte(feedback.Attachments), &attachments); err != nil {
		t.Fatal(err)
	}
	if len(attachments) != 1 || attachments[0].Name != "photo.png" || attachments[0].Type != "image" || attachments[0].Size != 42 || attachments[0].MimeType != "image/png" || attachments[0].URL != "/api/feedbacks/"+feedback.ID+"/attachments/owned/file" {
		t.Fatalf("non-canonical attachment: %+v", attachments)
	}
	for _, actor := range []*model.User{user, admin} {
		if _, err := svc.PrepareFeedbackAttachmentDelivery(actor, feedback.ID, "owned", ResourceAccessOptions{DescribeOnly: true}, ""); err != nil {
			t.Fatalf("authorized reader %s: %v", actor.ID, err)
		}
	}
	for _, test := range []struct {
		actor *model.User
		id    string
	}{
		{nil, "owned"}, {&model.User{ID: "other", Role: "user"}, "owned"}, {admin, "foreign"},
	} {
		if _, err := svc.PrepareFeedbackAttachmentDelivery(test.actor, feedback.ID, test.id, ResourceAccessOptions{DescribeOnly: true}, ""); err == nil {
			t.Fatalf("unauthorized reader accepted: %+v", test)
		}
	}
}

func TestFeedbackReferencesSurviveArchiveAndAssetPurgeUntilDeleted(t *testing.T) {
	svc, admin, user, db := newTestFeedbackService(t)
	resource := model.Resource{ID: "attached", UserID: user.ID, Kind: "image", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "photo.png", MimeType: "image/png"}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	feedback, err := svc.SubmitFeedback(user, SubmitFeedbackRequest{Title: "retained", Content: "test", Attachments: `[{"id":"attached"}]`})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdminArchiveFeedback(admin, feedback.ID, true); err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.repo.ResourceReferenceSnapshot(user.ID, "", []string{resource.ID})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ref := range snapshot.Direct {
		found = found || ref.Kind == "反馈" && ref.ID == feedback.ID && ref.ResourceID == resource.ID
	}
	if !found {
		t.Fatal("archived feedback attachment missing from references")
	}
	if err := svc.repo.DeleteDetachedResources([]model.Resource{resource}, nil); !errors.Is(err, repository.ErrResourceCleanupStillReferenced) {
		t.Fatalf("orphan cleanup should preserve attachment: %v", err)
	}
	asset := model.Asset{ID: "asset", UserID: user.ID, Kind: "image", PayloadJSON: `{"url":"/api/resources/attached/file"}`}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.deleteUserAssetWithResources(user.ID, asset.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.repo.ResourceForUser(user.ID, resource.ID); err != nil {
		t.Fatalf("asset purge removed feedback attachment: %v", err)
	}
	if err := svc.AdminDeleteFeedback(admin, feedback.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PrepareFeedbackAttachmentDelivery(admin, feedback.ID, resource.ID, ResourceAccessOptions{DescribeOnly: true}, ""); err == nil {
		t.Fatal("deleted feedback URL remained accessible")
	}
	if _, err := svc.repo.ResourceForUser(user.ID, resource.ID); err != nil {
		t.Fatal("feedback deletion must not immediately delete a shared resource")
	}
	if err := svc.repo.DeleteDetachedResources([]model.Resource{resource}, nil); err != nil {
		t.Fatalf("unreferenced attachment can now be cleaned: %v", err)
	}
}

func TestFeedbackManagementRequiresAdmin(t *testing.T) {
	svc, _, user, _ := newTestFeedbackService(t)
	for _, actor := range []*model.User{nil, user} {
		if _, err := svc.AdminListFeedbacks(actor, repository.FeedbackFilter{}); err == nil {
			t.Fatal("list requires admin")
		}
		if _, err := svc.AdminUpdateFeedback(actor, "missing", AdminUpdateFeedbackRequest{}); err == nil {
			t.Fatal("update requires admin")
		}
		if _, err := svc.AdminArchiveFeedback(actor, "missing", true); err == nil {
			t.Fatal("archive requires admin")
		}
		if err := svc.AdminDeleteFeedback(actor, "missing"); err == nil {
			t.Fatal("delete requires admin")
		}
	}
	if _, err := svc.SubmitFeedback(&model.User{}, SubmitFeedbackRequest{Title: "test", Content: strings.Repeat("a", 10)}); err == nil {
		t.Fatal("blank user ID must be rejected")
	}
}
