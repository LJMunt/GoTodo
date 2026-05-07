package service

import (
	"GoToDo/internal/logging"
	"GoToDo/internal/models"
	"GoToDo/internal/repository"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateTaskParams struct {
	WorkspaceID int64
	CreatorID   int64
	ProjectID   int64
	Title       string
	Description *string
	DueAt       *time.Time
	RepeatEvery *int
	RepeatUnit  *string
	AssignedTo  *string // Public ID
}

type TaskService interface {
	CreateTask(ctx context.Context, params CreateTaskParams) (*models.Task, error)
	GetTask(ctx context.Context, workspaceID, taskID int64) (*models.Task, error)
	UpdateTask(ctx context.Context, workspaceID, taskID int64, params UpdateTaskParams) (*models.Task, error)
	DeleteTask(ctx context.Context, workspaceID, taskID int64) error
	ListProjectTasks(ctx context.Context, workspaceID, projectID int64) ([]*models.Task, error)
	EnsureOccurrencesUpTo(ctx context.Context, db repository.DBTX, workspaceID, taskID int64, to time.Time) error
	ListTaskOccurrences(ctx context.Context, workspaceID, taskID int64, from, to time.Time) ([]*models.Occurrence, error)
	UpdateTaskOccurrence(ctx context.Context, workspaceID, taskID, occID int64, completed bool) (*models.Occurrence, error)
	GetTaskTags(ctx context.Context, workspaceID, taskID int64) ([]*models.Tag, error)
	UpdateTaskTags(ctx context.Context, workspaceID, taskID int64, tagIDs []int64) ([]*models.Tag, error)
}

type UpdateTaskParams struct {
	Title           *string
	Description     *string
	DueAt           *time.Time
	ClearDueAt      bool
	Completed       *bool
	RepeatEvery     *int
	RepeatUnit      *string
	ClearRepeat     bool
	AssignedTo      *string
	ClearAssignedTo bool
	ClosedBy        *int64
}

type taskService struct {
	pool           *pgxpool.Pool
	taskRepo       repository.TaskRepository
	projectRepo    repository.ProjectRepository
	userRepo       repository.UserRepository
	workspaceRepo  repository.WorkspaceRepository
	occurrenceRepo repository.OccurrenceRepository
	tagRepo        repository.TagRepository
}

func NewTaskService(
	pool *pgxpool.Pool,
	taskRepo repository.TaskRepository,
	projectRepo repository.ProjectRepository,
	userRepo repository.UserRepository,
	workspaceRepo repository.WorkspaceRepository,
	occurrenceRepo repository.OccurrenceRepository,
	tagRepo repository.TagRepository,
) TaskService {
	return &taskService{
		pool:           pool,
		taskRepo:       taskRepo,
		projectRepo:    projectRepo,
		userRepo:       userRepo,
		workspaceRepo:  workspaceRepo,
		occurrenceRepo: occurrenceRepo,
		tagRepo:        tagRepo,
	}
}

func (s *taskService) CreateTask(ctx context.Context, p CreateTaskParams) (*models.Task, error) {
	// 1. Validation
	if p.Title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	recurring := p.RepeatEvery != nil && p.RepeatUnit != nil
	if (p.RepeatEvery == nil) != (p.RepeatUnit == nil) {
		return nil, fmt.Errorf("%w: repeat_every and repeat_unit must be set together", ErrInvalidInput)
	}
	if p.RepeatEvery != nil && *p.RepeatEvery <= 0 {
		return nil, fmt.Errorf("%w: repeat_every must be > 0", ErrInvalidInput)
	}
	if recurring && p.DueAt == nil {
		return nil, fmt.Errorf("%w: due_at is required for recurring tasks", ErrInvalidInput)
	}

	// 2. Resolve assignee
	var assignedToID *int64
	if p.AssignedTo != nil && *p.AssignedTo != "" {
		id, err := s.resolveAssignedTo(ctx, s.pool, p.WorkspaceID, *p.AssignedTo)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
		}
		assignedToID = id
	}

	// 3. Start Transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// 4. Verify Project
	ok, err := s.projectRepo.Exists(ctx, tx, p.WorkspaceID, p.ProjectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: project not found", ErrNotFound)
	}

	// 5. Prepare Task model
	task := &models.Task{
		WorkspaceID: p.WorkspaceID,
		ProjectID:   p.ProjectID,
		Title:       p.Title,
		Description: p.Description,
		CreatedBy:   p.CreatorID,
		AssignedTo:  assignedToID,
	}

	if recurring {
		ra := p.DueAt.UTC()
		task.RecurrenceStartAt = &ra
		task.NextDueAt = &ra
		task.RepeatEvery = p.RepeatEvery
		task.RepeatUnit = p.RepeatUnit
	} else if p.DueAt != nil {
		d := p.DueAt.UTC()
		task.DueAt = &d
	}

	// 6. Create Task
	if err := s.taskRepo.Create(ctx, tx, task); err != nil {
		return nil, err
	}

	// 7. Handle recurrence
	if recurring {
		horizon := time.Now().UTC().AddDate(0, 0, 60)
		if err := s.EnsureOccurrencesUpTo(ctx, tx, p.WorkspaceID, task.ID, horizon); err != nil {
			// We can decide if this is fatal or not. In the original handler it wasn't inside the transaction or was it?
			// Actually in CreateTaskHandler it WAS inside the transaction.
			return nil, err
		}
		// Refresh next_due_at after generation
		err = tx.QueryRow(ctx, "SELECT next_due_at FROM tasks WHERE id=$1", task.ID).Scan(&task.NextDueAt)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *taskService) GetTask(ctx context.Context, workspaceID, taskID int64) (*models.Task, error) {
	task, err := s.taskRepo.GetByID(ctx, s.pool, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrNotFound
	}

	// Check project visibility (the repo GetByID doesn't check project)
	ok, err := s.taskRepo.IsVisible(ctx, s.pool, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}

	if task.RepeatEvery != nil && task.RepeatUnit != nil {
		horizon := time.Now().UTC().AddDate(0, 0, 60)
		_ = s.EnsureOccurrencesUpTo(ctx, s.pool, workspaceID, taskID, horizon)
		// Refresh next_due_at
		_ = s.pool.QueryRow(ctx, "SELECT next_due_at FROM tasks WHERE id=$1", taskID).Scan(&task.NextDueAt)
		task.DueAt = task.NextDueAt
		task.CompletedAt = nil
	}

	return task, nil
}

func (s *taskService) ListProjectTasks(ctx context.Context, workspaceID, projectID int64) ([]*models.Task, error) {
	// Verify project visibility
	ok, err := s.projectRepo.Exists(ctx, s.pool, workspaceID, projectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}

	return s.taskRepo.List(ctx, s.pool, workspaceID, &projectID)
}

func (s *taskService) DeleteTask(ctx context.Context, workspaceID, taskID int64) error {
	// Check visibility
	ok, err := s.taskRepo.IsVisible(ctx, s.pool, workspaceID, taskID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}

	return s.taskRepo.Delete(ctx, s.pool, workspaceID, taskID)
}

func (s *taskService) UpdateTask(ctx context.Context, workspaceID, taskID int64, p UpdateTaskParams) (*models.Task, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	task, err := s.taskRepo.GetByID(ctx, tx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrNotFound
	}

	// Additional visibility check for project
	ok, err := s.taskRepo.IsVisible(ctx, tx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}

	if p.Title != nil {
		if *p.Title == "" {
			return nil, fmt.Errorf("%w: title cannot be empty", ErrInvalidInput)
		}
		task.Title = *p.Title
	}
	if p.Description != nil {
		task.Description = p.Description
	}

	if p.ClearDueAt {
		task.DueAt = nil
	} else if p.DueAt != nil {
		d := p.DueAt.UTC()
		task.DueAt = &d
	}

	if p.Completed != nil {
		if *p.Completed {
			if task.CompletedAt == nil {
				now := time.Now().UTC()
				task.CompletedAt = &now
				task.ClosedBy = p.ClosedBy
			}
		} else {
			task.CompletedAt = nil
			task.ClosedBy = nil
		}
	}

	if p.ClearRepeat {
		task.RepeatEvery = nil
		task.RepeatUnit = nil
		task.RecurrenceStartAt = nil
		task.NextDueAt = nil
	} else {
		if (p.RepeatEvery == nil) != (p.RepeatUnit == nil) {
			// If only one is provided, we keep the other from the current task if it exists,
			// or fail if it doesn't.
			// The API handler was stricter: "repeat_every and repeat_unit must be set together"
			// But it allowed both to be nil (no change).
		}

		if p.RepeatEvery != nil {
			if *p.RepeatEvery <= 0 {
				return nil, fmt.Errorf("%w: repeat_every must be > 0", ErrInvalidInput)
			}
			task.RepeatEvery = p.RepeatEvery
		}
		if p.RepeatUnit != nil {
			task.RepeatUnit = p.RepeatUnit
		}

		// Validation of the final state
		if (task.RepeatEvery == nil) != (task.RepeatUnit == nil) {
			return nil, fmt.Errorf("%w: repeat_every and repeat_unit must be set together", ErrInvalidInput)
		}
	}

	if p.ClearAssignedTo {
		task.AssignedTo = nil
	} else if p.AssignedTo != nil {
		assignedToID, err := s.resolveAssignedTo(ctx, tx, workspaceID, *p.AssignedTo)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
		}
		task.AssignedTo = assignedToID
	}

	// If it became recurring just now
	if task.RepeatEvery != nil && task.RecurrenceStartAt == nil {
		if task.DueAt != nil {
			task.RecurrenceStartAt = task.DueAt
			task.NextDueAt = task.DueAt
			task.DueAt = nil
		} else {
			return nil, fmt.Errorf("%w: due_at is required for recurring tasks", ErrInvalidInput)
		}
	}

	if err := s.taskRepo.Update(ctx, tx, task); err != nil {
		return nil, err
	}

	if task.RepeatEvery != nil {
		horizon := time.Now().UTC().AddDate(0, 0, 60)
		if err := s.EnsureOccurrencesUpTo(ctx, tx, workspaceID, task.ID, horizon); err != nil {
			return nil, err
		}
		// Refresh task from DB to get next_due_at
		err = tx.QueryRow(ctx, "SELECT next_due_at FROM tasks WHERE id=$1", task.ID).Scan(&task.NextDueAt)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *taskService) EnsureOccurrencesUpTo(ctx context.Context, db repository.DBTX, workspaceID, taskID int64, to time.Time) error {
	l := logging.From(ctx)
	l.Debug().Int64("workspace_id", workspaceID).Int64("task_id", taskID).Time("to", to).Msg("ensuring task occurrences")

	task, err := s.taskRepo.GetByID(ctx, db, workspaceID, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found")
	}

	// Not recurring -> nothing to generate
	if task.RepeatEvery == nil || task.RepeatUnit == nil || *task.RepeatEvery <= 0 || *task.RepeatUnit == "" {
		return nil
	}

	anchor := time.Now().UTC()
	if task.RecurrenceStartAt != nil {
		anchor = task.RecurrenceStartAt.UTC()
	}

	// Find the latest existing due_at and occurrence_index
	lastDue, lastIndex, err := s.occurrenceRepo.GetMaxDueAndIndex(ctx, db, workspaceID, taskID)
	if err != nil {
		return err
	}

	step := func(t time.Time) (time.Time, error) {
		switch *task.RepeatUnit {
		case "day", "days":
			return t.AddDate(0, 0, *task.RepeatEvery), nil
		case "week", "weeks":
			return t.AddDate(0, 0, 7*(*task.RepeatEvery)), nil
		case "month", "months":
			return t.AddDate(0, *task.RepeatEvery, 0), nil
		default:
			return time.Time{}, fmt.Errorf("invalid repeat_unit: %s", *task.RepeatUnit)
		}
	}

	next := anchor
	nextIndex := int64(1)
	if lastDue != nil {
		next = lastDue.UTC()
		n, err := step(next)
		if err != nil {
			return err
		}
		next = n
	}
	if lastIndex != nil {
		nextIndex = *lastIndex + 1
	}
	to = to.UTC()
	count := 0

	// First, generate occurrences up to the requested window `to`
	for !next.After(to) {
		inserted, idx, err := s.occurrenceRepo.Insert(ctx, db, workspaceID, taskID, next, nextIndex)
		if err != nil {
			return err
		}
		if inserted {
			count++
		}
		nextIndex = idx + 1

		n, err := step(next)
		if err != nil {
			return err
		}
		next = n
	}

	// If fewer than 3 occurrences were generated, extend beyond `to` to reach at least 3
	for count < 3 {
		inserted, idx, err := s.occurrenceRepo.Insert(ctx, db, workspaceID, taskID, next, nextIndex)
		if err != nil {
			return err
		}
		if inserted {
			count++
		}
		nextIndex = idx + 1

		n, err := step(next)
		if err != nil {
			return err
		}
		next = n
		if count > 100 { // safety guard
			break
		}
	}

	if count > 0 {
		l.Debug().Int("generated", count).Msg("task occurrences generated")
	}

	now := time.Now().UTC()
	nextDue, err := s.occurrenceRepo.GetMinDueUncompleted(ctx, db, workspaceID, taskID, now)
	if err != nil {
		return err
	}

	_, _ = db.Exec(ctx,
		`UPDATE tasks SET next_due_at=$1, updated_at=now()
		 WHERE id=$2 AND workspace_id=$3`,
		nextDue, taskID, workspaceID,
	)

	return nil
}

func (s *taskService) ListTaskOccurrences(ctx context.Context, workspaceID, taskID int64, from, to time.Time) ([]*models.Occurrence, error) {
	// Only recurring tasks have occurrences.
	isRec, err := s.taskRepo.IsRecurringVisible(ctx, s.pool, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	if !isRec {
		return nil, ErrNotFound
	}

	// Lazy-generate up to `to` so the range query is complete.
	if err := s.EnsureOccurrencesUpTo(ctx, s.pool, workspaceID, taskID, to); err != nil {
		return nil, err
	}

	return s.occurrenceRepo.List(ctx, s.pool, workspaceID, taskID, from, to)
}

func (s *taskService) UpdateTaskOccurrence(ctx context.Context, workspaceID, taskID, occID int64, completed bool) (*models.Occurrence, error) {
	// Ensure this task is a visible recurring task.
	isRec, err := s.taskRepo.IsRecurringVisible(ctx, s.pool, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	if !isRec {
		return nil, ErrNotFound
	}

	var completedAt *time.Time
	if completed {
		now := time.Now().UTC()
		completedAt = &now
	}

	occ, err := s.occurrenceRepo.UpdateCompletion(ctx, s.pool, workspaceID, taskID, occID, completedAt)
	if err != nil {
		return nil, err
	}
	if occ == nil {
		return nil, ErrNotFound
	}

	// Best-effort: keep next_due_at fresh without cron
	_ = s.EnsureOccurrencesUpTo(ctx, s.pool, workspaceID, taskID, time.Now().UTC().AddDate(0, 0, 60))

	return occ, nil
}

func (s *taskService) GetTaskTags(ctx context.Context, workspaceID, taskID int64) ([]*models.Tag, error) {
	ok, err := s.taskRepo.IsVisible(ctx, s.pool, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}

	return s.tagRepo.ListByTaskID(ctx, s.pool, workspaceID, taskID)
}

func (s *taskService) UpdateTaskTags(ctx context.Context, workspaceID, taskID int64, tagIDs []int64) ([]*models.Tag, error) {
	// 1. Start transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// 2. Check visibility
	ok, err := s.taskRepo.IsVisible(ctx, tx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}

	// 3. Validate tags exist in workspace
	ok, err = s.tagRepo.ValidateTagsExist(ctx, tx, workspaceID, tagIDs)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: one or more tag_ids are invalid", ErrInvalidInput)
	}

	// 4. Assign tags
	if err := s.tagRepo.AssignToTask(ctx, tx, workspaceID, taskID, tagIDs); err != nil {
		return nil, err
	}

	// 5. Fetch updated tags
	tags, err := s.tagRepo.ListByTaskID(ctx, tx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return tags, nil
}

func (s *taskService) resolveAssignedTo(ctx context.Context, db repository.DBTX, workspaceID int64, publicID string) (*int64, error) {
	user, err := s.userRepo.GetByPublicID(ctx, db, publicID)
	if err != nil {
		return nil, err
	}
	if user == nil || !user.IsActive {
		return nil, errors.New("user not found or inactive")
	}

	ws, err := s.workspaceRepo.GetByID(ctx, db, workspaceID)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		return nil, errors.New("workspace not found")
	}

	if ws.Type == models.WorkspaceTypeUser {
		if ws.UserID != nil && *ws.UserID == user.ID {
			return &user.ID, nil
		}
		return nil, errors.New("user is not the owner of this personal workspace")
	} else if ws.Type == models.WorkspaceTypeOrg {
		if ws.OrgID == nil {
			return nil, errors.New("invalid organization workspace")
		}
		isMember, err := s.userRepo.IsMemberOfOrg(ctx, db, *ws.OrgID, user.ID)
		if err != nil {
			return nil, err
		}
		if isMember {
			return &user.ID, nil
		}
		return nil, errors.New("user is not a member of this organization")
	}

	return nil, errors.New("invalid workspace type")
}
