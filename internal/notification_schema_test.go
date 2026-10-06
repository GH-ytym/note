package internal

import (
	"testing"
	"time"

	"note/internal/model"
)

func TestNotificationSchemaAndRepeatMigration(t *testing.T) {
	db, owner, member, item := groupSchemaFixture(t)
	if !db.Migrator().HasTable(&model.Notification{}) {
		t.Fatal("startup did not create notifications")
	}
	createdAt := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	readAt := createdAt.Add(time.Hour)
	request := model.GroupJoinRequest{
		GroupID: item.ID, Kind: model.Application, SenderID: member.ID,
		ReceiverID: owner.ID, CreatedAt: createdAt,
	}
	if err := db.Omit("Group", "Sender", "Receiver").Create(&request).Error; err != nil {
		t.Fatal(err)
	}
	notice := model.Notification{
		ReceiverID: owner.ID, ActorID: member.ID, GroupID: item.ID,
		JoinRequestID: &request.ID, Type: model.JoinRequested,
		ReadAt: &readAt, CreatedAt: createdAt,
	}
	if err := db.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&notice).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := migrateDatabase(db); err != nil {
			t.Fatal(err)
		}
	}
	var saved model.Notification
	if err := db.Preload("Receiver").Preload("Actor").Preload("Group").Preload("JoinRequest").
		First(&saved, notice.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.ReceiverID != owner.ID || saved.ActorID != member.ID || saved.GroupID != item.ID ||
		saved.Type != model.JoinRequested || !saved.CreatedAt.Equal(createdAt) ||
		saved.ReadAt == nil || !saved.ReadAt.Equal(readAt) ||
		saved.JoinRequestID == nil || *saved.JoinRequestID != request.ID {
		t.Fatalf("repeat migration changed notification: %+v", saved)
	}
	if saved.Receiver == nil || saved.Receiver.ID != owner.ID ||
		saved.Actor == nil || saved.Actor.ID != member.ID ||
		saved.Group == nil || saved.Group.ID != item.ID ||
		saved.JoinRequest == nil || saved.JoinRequest.ID != request.ID {
		t.Fatalf("notification associations missing: %+v", saved)
	}
}

func TestNotificationSchemaRequestDeletionAndGroupCascade(t *testing.T) {
	db, owner, member, item := groupSchemaFixture(t)
	request := model.GroupJoinRequest{
		GroupID: item.ID, Kind: model.Application, SenderID: member.ID, ReceiverID: owner.ID,
	}
	if err := db.Omit("Group", "Sender", "Receiver").Create(&request).Error; err != nil {
		t.Fatal(err)
	}
	notices := []model.Notification{
		{ReceiverID: owner.ID, ActorID: member.ID, GroupID: item.ID, Type: model.Joined},
		{ReceiverID: owner.ID, ActorID: member.ID, GroupID: item.ID, Type: model.JoinRequested, JoinRequestID: &request.ID},
	}
	if err := db.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&notices).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&request).Error; err != nil {
		t.Fatal(err)
	}
	var saved model.Notification
	if err := db.First(&saved, notices[1].ID).Error; err != nil || saved.JoinRequestID != nil {
		t.Fatalf("request deletion did not clear reference: %+v err=%v", saved, err)
	}
	var count int64
	if err := db.Model(&model.Notification{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("request deletion removed notifications: count=%d err=%v", count, err)
	}
	if err := db.Delete(&item).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Notification{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("group deletion did not clear notifications: count=%d err=%v", count, err)
	}
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("group deletion changed users: count=%d err=%v", count, err)
	}
}
