package event

import (
	"context"
	"errors"
	apperrors "note/internal/errors"
	"note/internal/model"
	"note/internal/utils"
	"strings"
	"testing"
	"time"
)

type mockEventRepository struct {
	createCalls int
	created     *model.Event
	createErr   error
}

func (m *mockEventRepository) Create(_ context.Context, item *model.Event) error {
	m.createCalls++
	if m.createErr != nil {
		return m.createErr
	}
	item.ID = 1
	created := *item
	m.created = &created
	return nil
}

func TestCreate(t *testing.T) {
	repo := &mockEventRepository{}
	service := NewService(repo)

	startsat := time.Date(
		2026, time.September, 2,
		10, 0, 0, 0,
		time.Local,
	)
	endsat := startsat.Add(time.Hour)
	title := "qwen是傻逼"
	content := "deepseek是傻逼"
	item, err := service.Create(context.Background(),
		CreateCommand{
			Title:    title,
			Content:  &content,
			Color:    "#ffffff",
			StartsAt: startsat,
			EndsAt:   endsat,
		})
	if err != nil {
		t.Fatalf("Create returned an error %v", err)
	}
	if repo.createCalls != 1 {
		t.Fatalf("Create called %d times, expected 1", repo.createCalls)
	}
	if item.ID != 1 {
		t.Fatalf("Create called ID %d, expected 1", item.ID)
	}
	if item.Title != title {
		t.Fatalf("Create called Title %s, expected %s", item.Title, title)
	}
	if item.Content == nil {
		t.Fatal("Create returned nil Content")
	}

	if *item.Content != content {
		t.Fatalf(
			"Create called Content %q, expected %q",
			*item.Content,
			content,
		)
	}
	if item.Color != "#FFFFFF" {
		t.Fatalf("Create called Color %s, expected %s", item.Color, "#FFFFFF")
	}
	if !item.StartsAt.Equal(startsat) {
		t.Fatalf("Create called StartsAt %v, expected %v", item.StartsAt, startsat)
	}
	if !item.EndsAt.Equal(endsat) {
		t.Fatalf("Create called EndsAt %v, expected %v", item.EndsAt, endsat)
	}
}

func TestCreateEventRejectsInvalidTimeRange(t *testing.T) {
	startsAt := time.Date(
		2026, time.September, 2,
		10, 0, 0, 0,
		time.Local,
	)
	endsAt := startsAt.Add(time.Hour)
	tests := []struct {
		name     string
		startsAt time.Time
		endsAt   time.Time
	}{
		{
			name:     "nil startsAt",
			startsAt: time.Time{},
			endsAt:   endsAt,
		},
		{
			name:     "nil endsAt",
			startsAt: startsAt,
			endsAt:   time.Time{},
		},
		{
			name:     "endsat==startsat",
			startsAt: startsAt,
			endsAt:   startsAt,
		},
		{
			name:     "ends before startsat",
			startsAt: endsAt,
			endsAt:   startsAt,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			//每轮都新建repo，createcalls不会受到上一个测试影响
			repo := &mockEventRepository{}
			service := NewService(repo)

			_, err := service.Create(
				context.Background(),
				CreateCommand{
					Title:    "abc",
					Color:    "#ffffff",
					StartsAt: test.startsAt,
					EndsAt:   test.endsAt,
				})
			if !errors.Is(err, apperrors.ErrInvalidEventTimeRange) {
				t.Fatalf("Create returned an error %v", err)
			}
			if repo.createCalls != 0 {
				t.Errorf("Create called %d times, expected 0", repo.createCalls)
			}
		})
	}
}

func TestInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		change  func(*CreateCommand)
		wantErr error
	}{
		{
			name: "empty title",
			change: func(command *CreateCommand) {
				command.Title = ""
			},
			wantErr: apperrors.ErrTitleRequired,
		},
		{
			name: "content more than 500 characters",
			change: func(command *CreateCommand) {
				content := strings.Repeat("傻逼", 500)
				command.Content = &content
			},
			wantErr: apperrors.ErrEventInvalidContent,
		},
		{
			name: "invalid color",
			change: func(command *CreateCommand) {
				command.Color = "#red"
			},
			wantErr: apperrors.ErrEventInvalidColor,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			//同样的，每轮都创建新的repo和validCmd防止上一次结果影响这一次
			repo := &mockEventRepository{}
			service := NewService(repo)
			validCmd := validCreateCommand()
			test.change(&validCmd)
			_, err := service.Create(context.Background(), validCmd)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Create() error= %v,want %v",
					err,
					test.wantErr,
				)
			}
			if repo.createCalls != 0 {
				t.Errorf("Create called %d times, expected 0", repo.createCalls)
			}
		})
	}
}

func TestCreateReturnsRepositoryError(t *testing.T) {
	repositoryErr := errors.New("database unavailable")

	repo := &mockEventRepository{
		createErr: repositoryErr,
	}
	service := NewService(repo)
	//command合法所以应该被调用一次
	//repo模拟数据库故障，返回repositoryErr
	//service应该向上传递这个错误而不是吞掉
	command := validCreateCommand()

	_, err := service.Create(
		context.Background(),
		command,
	)

	if !errors.Is(err, repositoryErr) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			repositoryErr,
		)
	}

	if repo.createCalls != 1 {
		t.Errorf(
			"repository Create calls = %d, want 1",
			repo.createCalls,
		)
	}
}

func TestCreatePreservesOptionalContent(t *testing.T) {
	tests := []struct {
		name    string
		content *string
		wantNil bool
		want    string
	}{
		{
			name:    "missing content remains nil",
			content: nil,
			wantNil: true,
		},
		{
			name:    "blank content remains empty",
			content: eventStringPointer("   "),
			want:    "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &mockEventRepository{}
			service := NewService(repo)
			command := validCreateCommand()
			command.Content = test.content

			item, err := service.Create(context.Background(), command)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}

			if test.wantNil {
				if item.Content != nil {
					t.Fatalf("Content = %q, want nil", *item.Content)
				}
				return
			}

			if item.Content == nil {
				t.Fatal("Content = nil, want non-nil")
			}
			if *item.Content != test.want {
				t.Errorf("Content = %q, want %q", *item.Content, test.want)
			}
		})
	}
}

func TestCreateAssignsColorWhenMissing(t *testing.T) {
	repo := &mockEventRepository{}
	service := NewService(repo)
	command := validCreateCommand()
	command.Color = ""

	item, err := service.Create(context.Background(), command)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, valid := utils.NormalizeHexColor(item.Color); !valid {
		t.Errorf("generated Color = %q, want a valid hex color", item.Color)
	}
	if repo.createCalls != 1 {
		t.Errorf("repository Create calls = %d, want 1", repo.createCalls)
	}
}

// 给个正确的create command
func validCreateCommand() CreateCommand {
	startsAt := time.Date(
		2026, time.September, 3,
		10, 0, 0, 0,
		time.Local,
	)

	return CreateCommand{
		Title:    "项目会议",
		Color:    "#AABBCC",
		StartsAt: startsAt,
		EndsAt:   startsAt.Add(time.Hour),
	}
}

func eventStringPointer(value string) *string {
	return &value
}
