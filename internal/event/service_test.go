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
	getResult   model.Event
	getErr      error
	updateCalls int
	updateErr   error
	createCalls int
	created     *model.Event
	createErr   error
	listCalls   int
	listFrom    time.Time
	listTo      time.Time
	listResult  []model.Event
	listErr     error
}

func (m *mockEventRepository) Get(_ context.Context, _ uint) (model.Event, error) {
	return m.getResult, m.getErr
}
func (m *mockEventRepository) Update(_ context.Context, item *model.Event, version uint) error {
	m.updateCalls++
	if m.updateErr != nil {
		return m.updateErr
	}
	item.Version = version + 1
	m.getResult = *item
	return nil
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

func (m *mockEventRepository) ListInRange(
	_ context.Context,
	from time.Time,
	to time.Time,
) ([]model.Event, error) {
	m.listCalls++
	m.listFrom = from
	m.listTo = to
	return m.listResult, m.listErr
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
			Title:      title,
			Content:    &content,
			Color:      "#ffffff",
			StartsAt:   startsat,
			EndsAt:     endsat,
			RepeatMode: model.RepeatOnce,
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
					Title:      "abc",
					Color:      "#ffffff",
					StartsAt:   test.startsAt,
					EndsAt:     test.endsAt,
					RepeatMode: model.RepeatOnce,
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
		{
			name: "invalid repeat mode",
			change: func(command *CreateCommand) {
				command.RepeatMode = "yearly"
			},
			wantErr: apperrors.ErrInvalidRepeatMode,
		},
		{
			name: "custom repeat without dates",
			change: func(command *CreateCommand) {
				command.RepeatMode = model.RepeatCustom
			},
			wantErr: apperrors.ErrCustomDatesRequired,
		},
		{
			name: "non-custom repeat with dates",
			change: func(command *CreateCommand) {
				command.CustomDates = []time.Time{time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC)}
			},
			wantErr: apperrors.ErrCustomDatesNotAllowed,
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

func TestCreateStoresUniqueCustomDates(t *testing.T) {
	repo := &mockEventRepository{}
	service := NewService(repo)
	command := validCreateCommand()
	date := time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC)
	command.RepeatMode = model.RepeatCustom
	command.CustomDates = []time.Time{date, date}

	item, err := service.Create(context.Background(), command)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(item.CustomDates) != 1 || !item.CustomDates[0].Date.Equal(date) {
		t.Fatalf("CustomDates = %#v, want one %v", item.CustomDates, date)
	}
}

func TestListInRangeExpandsOccurrences(t *testing.T) {
	from := time.Date(2026, time.September, 9, 0, 0, 0, 0, time.Local)
	to := from.AddDate(0, 0, 1)
	content := "讨论项目"
	repo := &mockEventRepository{listResult: []model.Event{
		{
			ID:         2,
			Title:      "每日站会",
			Color:      "#AABBCC",
			StartsAt:   time.Date(2026, time.September, 1, 9, 0, 0, 0, time.Local),
			EndsAt:     time.Date(2026, time.September, 1, 9, 30, 0, 0, time.Local),
			RepeatMode: model.RepeatDaily,
			Version:    3,
		},
		{
			ID:         1,
			Title:      "早餐",
			Content:    &content,
			Color:      "#F3B51B",
			StartsAt:   time.Date(2026, time.September, 9, 8, 0, 0, 0, time.Local),
			EndsAt:     time.Date(2026, time.September, 9, 8, 45, 0, 0, time.Local),
			RepeatMode: model.RepeatOnce,
			Version:    1,
		},
	}}
	service := NewService(repo)

	items, err := service.ListInRange(context.Background(), from, to)
	if err != nil {
		t.Fatalf("ListInRange() error = %v", err)
	}
	if repo.listCalls != 1 {
		t.Fatalf("repository ListInRange calls = %d, want 1", repo.listCalls)
	}
	if !repo.listFrom.Equal(from) || !repo.listTo.Equal(to) {
		t.Errorf(
			"repository range = [%v, %v), want [%v, %v)",
			repo.listFrom,
			repo.listTo,
			from,
			to,
		)
	}
	if len(items) != 2 {
		t.Fatalf("items length = %d, want 2", len(items))
	}
	if items[0].EventID != 1 || items[0].Content != content || items[0].StartsAt.Hour() != 8 {
		t.Errorf("first occurrence = %#v", items[0])
	}
	if items[1].EventID != 2 || items[1].Content != "" || items[1].StartsAt.Hour() != 9 {
		t.Errorf("second occurrence = %#v", items[1])
	}
	if got := items[1].EndsAt.Sub(items[1].StartsAt); got != 30*time.Minute {
		t.Errorf("daily occurrence duration = %v, want 30m", got)
	}
}

func TestListInRangeLongWeeklyEventCanOverlapItself(t *testing.T) {
	from := time.Date(2026, time.September, 9, 0, 0, 0, 0, time.Local)
	start := time.Date(2026, time.August, 25, 10, 0, 0, 0, time.Local)
	repo := &mockEventRepository{listResult: []model.Event{{
		ID: 1, Title: "跨月周期", StartsAt: start, EndsAt: start.AddDate(0, 0, 10), RepeatMode: model.RepeatWeekly,
	}}}
	items, err := NewService(repo).ListInRange(context.Background(), from, from.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d occurrences, want 2", len(items))
	}
	for i, item := range items {
		wantStart := start.AddDate(0, 0, 7*(i+1))
		if !item.StartsAt.Equal(wantStart) || !item.EndsAt.Equal(wantStart.AddDate(0, 0, 10)) {
			t.Errorf("occurrence %d = %v-%v", i, item.StartsAt, item.EndsAt)
		}
	}
}

func TestListInRangeIncludesOccurrenceOverlappingStart(t *testing.T) {
	from := time.Date(2026, time.September, 9, 0, 0, 0, 0, time.Local)
	to := from.AddDate(0, 0, 1)
	repo := &mockEventRepository{listResult: []model.Event{{
		ID:         1,
		Title:      "跨日值班",
		StartsAt:   time.Date(2026, time.September, 1, 23, 0, 0, 0, time.Local),
		EndsAt:     time.Date(2026, time.September, 2, 1, 0, 0, 0, time.Local),
		RepeatMode: model.RepeatDaily,
	}}}
	service := NewService(repo)

	items, err := service.ListInRange(context.Background(), from, to)
	if err != nil {
		t.Fatalf("ListInRange() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items length = %d, want 2", len(items))
	}
	if !items[0].StartsAt.Equal(from.Add(-time.Hour)) || !items[0].EndsAt.Equal(from.Add(time.Hour)) {
		t.Errorf("overlapping occurrence = %v-%v", items[0].StartsAt, items[0].EndsAt)
	}
}

func TestListInRangeExpandsCustomDates(t *testing.T) {
	from := time.Date(2026, time.September, 9, 0, 0, 0, 0, time.Local)
	to := from.AddDate(0, 0, 1)
	repo := &mockEventRepository{listResult: []model.Event{{
		ID:         4,
		Title:      "自定义日程",
		StartsAt:   time.Date(2026, time.September, 1, 23, 0, 0, 0, time.Local),
		EndsAt:     time.Date(2026, time.September, 2, 1, 0, 0, 0, time.Local),
		RepeatMode: model.RepeatCustom,
		CustomDates: []model.EventDate{
			{Date: time.Date(2026, time.September, 8, 0, 0, 0, 0, time.UTC)},
			{Date: time.Date(2026, time.September, 9, 0, 0, 0, 0, time.UTC)},
		},
	}}}
	service := NewService(repo)

	items, err := service.ListInRange(context.Background(), from, to)
	if err != nil {
		t.Fatalf("ListInRange() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items length = %d, want 2", len(items))
	}
	if !items[0].StartsAt.Equal(from.Add(-time.Hour)) || !items[1].StartsAt.Equal(from.Add(23*time.Hour)) {
		t.Errorf("custom occurrence starts = %v, %v", items[0].StartsAt, items[1].StartsAt)
	}
}

func TestListInRangeRejectsInvalidStoredEvent(t *testing.T) {
	from := time.Date(2026, time.September, 9, 0, 0, 0, 0, time.Local)
	repo := &mockEventRepository{listResult: []model.Event{{
		ID:         7,
		StartsAt:   from,
		EndsAt:     from,
		RepeatMode: model.RepeatOnce,
	}}}
	service := NewService(repo)

	_, err := service.ListInRange(context.Background(), from, from.AddDate(0, 0, 1))
	if !errors.Is(err, apperrors.ErrInvalidEventTimeRange) {
		t.Fatalf("ListInRange() error = %v, want %v", err, apperrors.ErrInvalidEventTimeRange)
	}
}

func TestListInRangeRejectsInvalidRange(t *testing.T) {
	start := time.Date(2026, time.September, 9, 0, 0, 0, 0, time.Local)
	tests := []struct {
		name string
		from time.Time
		to   time.Time
	}{
		{name: "zero from", from: time.Time{}, to: start},
		{name: "zero to", from: start, to: time.Time{}},
		{name: "equal bounds", from: start, to: start},
		{name: "reversed bounds", from: start, to: start.Add(-time.Minute)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &mockEventRepository{}
			service := NewService(repo)

			_, err := service.ListInRange(context.Background(), test.from, test.to)
			if !errors.Is(err, apperrors.ErrInvalidCalendarRange) {
				t.Fatalf("ListInRange() error = %v, want %v", err, apperrors.ErrInvalidCalendarRange)
			}
			if repo.listCalls != 0 {
				t.Errorf("repository ListInRange calls = %d, want 0", repo.listCalls)
			}
		})
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
		Title:      "项目会议",
		Color:      "#AABBCC",
		StartsAt:   startsAt,
		EndsAt:     startsAt.Add(time.Hour),
		RepeatMode: model.RepeatOnce,
	}
}

func eventStringPointer(value string) *string {
	return &value
}
