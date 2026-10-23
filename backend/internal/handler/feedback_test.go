package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"yingce/backend/internal/app"
	"yingce/backend/internal/auth"
	"yingce/backend/internal/database"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestFeedbackHandler(t *testing.T) (*gin.Engine, *model.User, *model.User) {
	t.Helper()
	gin.SetMode(gin.TestMode)
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
	dataDir := t.TempDir()
	svc := app.New(repo, dataDir)
	t.Cleanup(func() { _ = svc.Close() })

	admin := &model.User{ID: "admin_u1", Username: "admin", Role: "admin", Status: "active", CreatedAt: time.Now()}
	user := &model.User{ID: "normal_u1", Username: "normal", Email: "normal@example.com", Role: "user", Status: "active", CreatedAt: time.Now()}

	if err := db.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	other := &model.User{ID: "other_u1", Username: "other", Role: "user", Status: "active"}
	if err := db.Create(other).Error; err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "resources"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "resources", "attachment.png"), []byte("test-image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Resource{ID: "attachment", UserID: user.ID, Kind: "image", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "attachment.png", MimeType: "image/png", Size: 10}).Error; err != nil {
		t.Fatal(err)
	}

	// Create sessions
	db.Create(&model.AuthSession{ID: "sess_admin", UserID: admin.ID, TokenHash: auth.HashToken("admin-tok"), CreatedAt: time.Now(), ExpiresAt: time.Now().Add(24 * time.Hour)})
	db.Create(&model.AuthSession{ID: "sess_user", UserID: user.ID, TokenHash: auth.HashToken("user-tok"), CreatedAt: time.Now(), ExpiresAt: time.Now().Add(24 * time.Hour)})
	db.Create(&model.AuthSession{ID: "sess_other", UserID: other.ID, TokenHash: auth.HashToken("other-tok"), CreatedAt: time.Now(), ExpiresAt: time.Now().Add(24 * time.Hour)})

	r := gin.New()
	api := r.Group("/api")
	RegisterFeedbackRoutes(api, svc)

	return r, admin, user
}

func TestFeedbackAttachmentHTTPAuthorizationAndRange(t *testing.T) {
	router, _, _ := newTestFeedbackHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/feedbacks", bytes.NewBufferString(`{"title":"screen","content":"details","attachments":"[{\"id\":\"attachment\"}]"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "open_ai_canvas_session", Value: "sess_user.user-tok"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("submit: %d %s", response.Code, response.Body.String())
	}
	var created struct {
		Data model.UserFeedback `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	var attachments []model.FeedbackAttachment
	if err := json.Unmarshal([]byte(created.Data.Attachments), &attachments); err != nil || len(attachments) != 1 {
		t.Fatalf("attachment: %+v %v", attachments, err)
	}
	for _, test := range []struct {
		name, cookie, rangeHeader, suffix string
		status                            int
	}{
		{name: "owner", cookie: "sess_user.user-tok", status: 200},
		{name: "admin", cookie: "sess_admin.admin-tok", status: 200},
		{name: "range", cookie: "sess_admin.admin-tok", rangeHeader: "bytes=0-3", status: 206},
		{name: "anonymous", status: 401},
		{name: "other user", cookie: "sess_other.other-tok", status: 404},
		{name: "not an attachment", cookie: "sess_admin.admin-tok", suffix: "/api/feedbacks/" + created.Data.ID + "/attachments/unrelated/file", status: 404},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := attachments[0].URL
			if test.suffix != "" {
				path = test.suffix
			}
			request := httptest.NewRequest(http.MethodGet, path, nil)
			if test.cookie != "" {
				request.AddCookie(&http.Cookie{Name: "open_ai_canvas_session", Value: test.cookie})
			}
			request.Header.Set("Range", test.rangeHeader)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if test.status == 200 && response.Body.String() != "test-image" {
				t.Fatalf("unexpected bytes: %s", response.Body.String())
			}
			if test.status == 206 && response.Body.String() != "test" {
				t.Fatalf("unexpected range bytes: %s", response.Body.String())
			}
		})
	}
}

func TestFeedbackHTTPEndpoints(t *testing.T) {
	r, _, _ := newTestFeedbackHandler(t)

	// 1. Submit feedback
	payload := map[string]any{
		"type":        "bug",
		"title":       "测试反馈标题",
		"content":     "测试反馈正文内容描述详细信息",
		"contact":     "test@user.com",
		"attachments": "[]",
		"environment": "{}",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/feedbacks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "open_ai_canvas_session", Value: "sess_user.user-tok"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var submitResp struct {
		Code int                `json:"code"`
		Data model.UserFeedback `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &submitResp); err != nil {
		t.Fatalf("decode submit response: %v", err)
	}
	if submitResp.Data.ID == "" || submitResp.Data.Title != "测试反馈标题" {
		t.Fatalf("unexpected feedback data: %+v", submitResp.Data)
	}

	// 2. User gets my feedbacks
	reqMy := httptest.NewRequest(http.MethodGet, "/api/feedbacks/my", nil)
	reqMy.AddCookie(&http.Cookie{Name: "open_ai_canvas_session", Value: "sess_user.user-tok"})
	wMy := httptest.NewRecorder()
	r.ServeHTTP(wMy, reqMy)

	if wMy.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", wMy.Code, wMy.Body.String())
	}

	// 3. Admin gets feedbacks
	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/admin/feedbacks", nil)
	reqAdmin.AddCookie(&http.Cookie{Name: "open_ai_canvas_session", Value: "sess_admin.admin-tok"})
	wAdmin := httptest.NewRecorder()
	r.ServeHTTP(wAdmin, reqAdmin)

	if wAdmin.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", wAdmin.Code, wAdmin.Body.String())
	}

	// 4. Admin reply & patch
	patchPayload := map[string]any{
		"status":     "resolved",
		"adminReply": "已处理完毕",
	}
	patchBody, _ := json.Marshal(patchPayload)
	reqPatch := httptest.NewRequest(http.MethodPatch, "/api/admin/feedbacks/"+submitResp.Data.ID, bytes.NewReader(patchBody))
	reqPatch.Header.Set("Content-Type", "application/json")
	reqPatch.AddCookie(&http.Cookie{Name: "open_ai_canvas_session", Value: "sess_admin.admin-tok"})
	wPatch := httptest.NewRecorder()
	r.ServeHTTP(wPatch, reqPatch)

	if wPatch.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", wPatch.Code, wPatch.Body.String())
	}

	// 5. Admin archive feedback
	reqArchive := httptest.NewRequest(http.MethodPost, "/api/admin/feedbacks/"+submitResp.Data.ID+"/archive", nil)
	reqArchive.AddCookie(&http.Cookie{Name: "open_ai_canvas_session", Value: "sess_admin.admin-tok"})
	wArchive := httptest.NewRecorder()
	r.ServeHTTP(wArchive, reqArchive)

	if wArchive.Code != http.StatusOK {
		t.Fatalf("expected 200 on archive, got %d: %s", wArchive.Code, wArchive.Body.String())
	}

	// 6. Admin query with archived=true
	reqArchivedList := httptest.NewRequest(http.MethodGet, "/api/admin/feedbacks?archived=true", nil)
	reqArchivedList.AddCookie(&http.Cookie{Name: "open_ai_canvas_session", Value: "sess_admin.admin-tok"})
	wArchivedList := httptest.NewRecorder()
	r.ServeHTTP(wArchivedList, reqArchivedList)

	if wArchivedList.Code != http.StatusOK {
		t.Fatalf("expected 200 on query archived list, got %d: %s", wArchivedList.Code, wArchivedList.Body.String())
	}
}
