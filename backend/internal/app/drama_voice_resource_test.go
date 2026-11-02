package app

import (
	"errors"
	"testing"

	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

func TestDramaVoiceSampleRemainsReferencedWithoutVoiceLibraryEntry(t *testing.T) {
	svc, db := newMediaRecoveryTestService(t)
	image := model.Resource{ID: "portrait", UserID: "owner", Kind: "image", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "portrait.png"}
	sample := model.Resource{ID: "voice", UserID: "owner", Kind: "audio", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "voice.wav"}
	for _, row := range []any{
		&model.Project{ID: "drama", UserID: "owner", Name: "短剧"}, &image, &sample,
		&model.DramaCast{ID: "character", ProjectID: "drama", Kind: "character", Name: "角色", ImageResourceID: image.ID},
		&model.Asset{ID: "voice-asset", UserID: "owner", Kind: "audio", PayloadJSON: `{"url":"/api/resources/voice/file"}`},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.repo.UpdateDramaCastFields("character", map[string]any{"voice_sample_resource_id": sample.ID}); err != nil {
		t.Fatal(err)
	}
	for _, ids := range [][]string{{sample.ID}, {image.ID}, {sample.ID, image.ID}, nil} {
		refs, err := svc.repo.DramaResourceReferences("owner", ids)
		want := len(ids)
		if ids == nil {
			want = 2
		}
		if err != nil || len(refs) != want {
			t.Fatalf("resource reference filter %v: %+v %v", ids, refs, err)
		}
		if len(ids) == 1 && refs[0].ResourceID != ids[0] {
			t.Fatalf("filtered voice/image query returned an unrelated resource: %+v", refs)
		}
	}
	if refs, err := svc.repo.DramaResourceReferences("someone", []string{sample.ID}); err != nil || len(refs) != 0 {
		t.Fatalf("cross-account voice references: %+v %v", refs, err)
	}
	if err := svc.repo.DeleteDetachedResources([]model.Resource{sample}, nil); !errors.Is(err, repository.ErrResourceCleanupStillReferenced) {
		t.Fatalf("cleanup deleted a bound voice: %v", err)
	}
	if err := svc.deleteUserAssetWithResources("owner", "voice-asset", true); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.repo.Resource(sample.ID); err != nil {
		t.Fatalf("asset purge removed a bound voice: %v", err)
	}
	if err := svc.repo.UpdateDramaCastFields("character", map[string]any{"voice_sample_resource_id": ""}); err != nil {
		t.Fatal(err)
	}
	if refs, err := svc.repo.DramaResourceReferences("owner", []string{sample.ID}); err != nil || len(refs) != 0 {
		t.Fatalf("cleared voice still holds a resource reference: %+v %v", refs, err)
	}
}
