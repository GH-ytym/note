package internal

import (
	"testing"
	"time"

	"note/internal/model"
)

func TestGroupJoinSchemaAndRepeatMigration(t *testing.T) {
	db, owner, member, item := groupSchemaFixture(t)
	if !db.Migrator().HasTable(&model.GroupJoinRequest{}) {
		t.Fatal("startup did not create group_join_requests")
	}
	if err := db.Create(&[]model.GroupMember{{GroupID: item.ID, UserID: owner.ID}, {GroupID: item.ID, UserID: member.ID}}).Error; err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	request := model.GroupJoinRequest{
		GroupID: item.ID, Kind: model.Invitation, SenderID: owner.ID, ReceiverID: member.ID, CreatedAt: createdAt,
	}
	if err := db.Omit("Group", "Sender", "Receiver").Create(&request).Error; err != nil {
		t.Fatal(err)
	}
	if request.Status != model.Pending || request.CreatedAt.IsZero() || request.HandledAt != nil {
		t.Fatalf("unexpected initial request: %+v", request)
	}
	for i := 0; i < 2; i++ {
		if err := migrateDatabase(db); err != nil {
			t.Fatal(err)
		}
	}
	var saved model.GroupJoinRequest
	if err := db.Preload("Group").Preload("Sender").Preload("Receiver").First(&saved, request.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != model.Pending || !saved.CreatedAt.Equal(createdAt) || saved.HandledAt != nil {
		t.Fatalf("repeat migration changed request: %+v", saved)
	}
	if saved.Group == nil || saved.Group.ID != item.ID || saved.Sender == nil || saved.Sender.ID != owner.ID || saved.Receiver == nil || saved.Receiver.ID != member.ID {
		t.Fatalf("request associations missing: %+v", saved)
	}
	var memberCount int64
	if err := db.Model(&model.GroupMember{}).Where("group_id = ?", item.ID).Count(&memberCount).Error; err != nil || memberCount != 2 {
		t.Fatalf("migration changed memberships: count=%d err=%v", memberCount, err)
	}
}

func TestGroupJoinSchemaConstraintsAndCascade(t *testing.T) {
	db, owner, member, item := groupSchemaFixture(t)
	for _, kind := range []model.Kind{model.Invitation, model.Application} {
		for _, status := range []model.Status{model.Pending, model.Accepted, model.Rejected, model.Cancelled} {
			request := model.GroupJoinRequest{GroupID: item.ID, Kind: kind, SenderID: owner.ID, ReceiverID: member.ID, Status: status}
			if err := db.Omit("Group", "Sender", "Receiver").Create(&request).Error; err != nil {
				t.Fatalf("valid request rejected: %v", err)
			}
		}
	}
	for _, tc := range []struct {
		name string
		edit func(*model.GroupJoinRequest)
	}{
		{"invalid kind", func(r *model.GroupJoinRequest) { r.Kind = "invalid" }},
		{"missing kind", func(r *model.GroupJoinRequest) { r.Kind = "" }},
		{"invalid status", func(r *model.GroupJoinRequest) { r.Status = "invalid" }},
		{"unknown group", func(r *model.GroupJoinRequest) { r.GroupID = 999999 }},
		{"unknown sender", func(r *model.GroupJoinRequest) { r.SenderID = 999999 }},
		{"unknown receiver", func(r *model.GroupJoinRequest) { r.ReceiverID = 999999 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := model.GroupJoinRequest{GroupID: item.ID, Kind: model.Invitation, SenderID: owner.ID, ReceiverID: member.ID, Status: model.Pending}
			tc.edit(&request)
			if err := db.Omit("Group", "Sender", "Receiver").Create(&request).Error; err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
	// 仍被邀请记录引用的用户不能直接删除；删除群则级联清理该群申请。
	if err := db.Delete(&member).Error; err == nil {
		t.Fatal("deleted referenced receiver")
	}
	if err := db.Delete(&item).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.GroupJoinRequest{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("group deletion did not clear requests: count=%d err=%v", count, err)
	}
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("group deletion changed users: count=%d err=%v", count, err)
	}
}
