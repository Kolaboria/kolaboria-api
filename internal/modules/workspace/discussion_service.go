package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/workspaceactivity"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
)

var (
	ErrDiscussionProjectNotWritable  = fmt.Errorf("%w: discussions can only be changed while the project is in progress", sharedErrors.ErrConflict)
	ErrDiscussionIdempotencyConflict = fmt.Errorf("%w: idempotency key was already used with different content", sharedErrors.ErrConflict)
	ErrDiscussionReplyDeleted        = fmt.Errorf("%w: deleted replies cannot receive new replies", sharedErrors.ErrConflict)
)

type DiscussionService struct{ repository DiscussionRepository }

func NewDiscussionService(repository DiscussionRepository) *DiscussionService {
	return &DiscussionService{repository: repository}
}

func (service *DiscussionService) List(ctx context.Context, actorID uuid.UUID, slug string, requestedPage, requestedLimit int) ([]DiscussionListItemResponse, project.ProjectPaginationMeta, error) {
	access, err := service.resolveAccess(ctx, actorID, slug)
	if err != nil {
		return nil, project.ProjectPaginationMeta{}, err
	}
	page := normalizeDiscussionPage(requestedPage)
	limit := normalizeDiscussionPageSize(requestedLimit)
	rows, total, err := service.repository.ListDiscussions(ctx, access.ID, page, limit)
	if err != nil {
		return nil, project.ProjectPaginationMeta{}, fmt.Errorf("list workspace discussions: %w", err)
	}
	items := make([]DiscussionListItemResponse, 0, len(rows))
	for _, row := range rows {
		canWrite := canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID))
		items = append(items, mapDiscussionListItem(row, actorID, ownerID(access), canWrite))
	}
	return items, discussionPaginationMeta(page, limit, total), nil
}

func (service *DiscussionService) Get(ctx context.Context, actorID uuid.UUID, slug, discussionIDValue string) (*DiscussionResponse, error) {
	access, err := service.resolveAccess(ctx, actorID, slug)
	if err != nil {
		return nil, err
	}
	discussionID, err := parseDiscussionID(discussionIDValue)
	if err != nil {
		return nil, err
	}
	row, err := service.repository.GetDiscussion(ctx, access.ID, discussionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, sharedErrors.ErrNotFound
		}
		return nil, fmt.Errorf("load workspace discussion: %w", err)
	}
	response := mapDiscussion(*row, actorID, ownerID(access), canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID)))
	return &response, nil
}

func (service *DiscussionService) Create(ctx context.Context, actorID uuid.UUID, slug, idempotencyKey string, request CreateDiscussionRequest) (*DiscussionResponse, bool, error) {
	title, err := normalizeDiscussionTitle(request.Title)
	if err != nil {
		return nil, false, err
	}
	body, err := normalizeDiscussionBody(request.Body)
	if err != nil {
		return nil, false, err
	}
	key, err := normalizeDiscussionIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, false, err
	}
	access, err := service.resolveAccess(ctx, actorID, slug)
	if err != nil {
		return nil, false, err
	}
	hash := discussionRequestHash(title, body)
	var response *DiscussionResponse
	created := false
	err = service.repository.WithDiscussionTransaction(ctx, func(tx DiscussionTx) error {
		access, err = tx.ValidateDiscussionWriter(ctx, access.ID, actorID)
		if err != nil {
			return err
		}
		existing, findErr := tx.FindDiscussionByIdempotency(ctx, access.ID, actorID, key)
		if findErr == nil {
			if existing.RequestHash == nil || *existing.RequestHash != hash {
				return ErrDiscussionIdempotencyConflict
			}
			if existing.DeletedAt != nil {
				return sharedErrors.ErrConflict
			}
			row, err := tx.GetDiscussion(ctx, access.ID, existing.ID, false)
			if err != nil {
				return err
			}
			mapped := mapDiscussion(*row, actorID, ownerID(access), canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID)))
			response = &mapped
			return nil
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("find idempotent workspace discussion: %w", findErr)
		}
		if !canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID)) {
			return ErrDiscussionProjectNotWritable
		}
		now := time.Now().UTC()
		keyCopy, hashCopy := key, hash
		discussion := &Discussion{
			ID: uuid.New(), ProjectID: access.ID, AuthorID: actorID, Title: title, Body: body,
			LastActivityAt: now, IdempotencyKey: &keyCopy, RequestHash: &hashCopy,
			CreatedAt: now, UpdatedAt: now,
		}
		inserted, err := tx.InsertDiscussion(ctx, discussion)
		if err != nil {
			return fmt.Errorf("create workspace discussion: %w", err)
		}
		if !inserted {
			existing, err = tx.FindDiscussionByIdempotency(ctx, access.ID, actorID, key)
			if err != nil {
				return fmt.Errorf("resolve concurrent discussion retry: %w", err)
			}
			if existing.RequestHash == nil || *existing.RequestHash != hash || existing.DeletedAt != nil {
				return ErrDiscussionIdempotencyConflict
			}
			discussion.ID = existing.ID
		} else {
			created = true
			if err := tx.AppendActivity(ctx, workspaceactivity.Input{ProjectID: access.ID, ActorID: actorID, SubjectType: "discussion", SubjectID: discussion.ID, Type: "discussion.created", At: now, Payload: map[string]any{"title_snapshot": title}}); err != nil {
				return err
			}
		}
		row, err := tx.GetDiscussion(ctx, access.ID, discussion.ID, false)
		if err != nil {
			return fmt.Errorf("load created workspace discussion: %w", err)
		}
		mapped := mapDiscussion(*row, actorID, ownerID(access), canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID)))
		response = &mapped
		return nil
	})
	if err != nil {
		return nil, false, mapDiscussionPersistenceError(err)
	}
	return response, created, nil
}

func (service *DiscussionService) Update(ctx context.Context, actorID uuid.UUID, slug, discussionIDValue string, request UpdateDiscussionRequest) (*DiscussionResponse, error) {
	discussionID, err := parseDiscussionID(discussionIDValue)
	if err != nil {
		return nil, err
	}
	access, err := service.resolveAccess(ctx, actorID, slug)
	if err != nil {
		return nil, err
	}
	if request.Title == nil && request.Body == nil {
		return nil, fmt.Errorf("%w: at least one discussion field must be updated", sharedErrors.ErrBadRequest)
	}
	var response *DiscussionResponse
	err = service.repository.WithDiscussionTransaction(ctx, func(tx DiscussionTx) error {
		access, err = tx.ValidateDiscussionWriter(ctx, access.ID, actorID)
		if err != nil {
			return err
		}
		if !canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID)) {
			return ErrDiscussionProjectNotWritable
		}
		row, err := tx.GetDiscussion(ctx, access.ID, discussionID, true)
		if err != nil {
			return err
		}
		if !canEditDiscussion(row.AuthorID, actorID) {
			return sharedErrors.ErrForbidden
		}
		title, body := row.Title, row.Body
		if request.Title != nil {
			title, err = normalizeDiscussionTitle(*request.Title)
			if err != nil {
				return err
			}
		}
		if request.Body != nil {
			body, err = normalizeDiscussionBody(*request.Body)
			if err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		if title != row.Title || body != row.Body {
			if err := tx.UpdateDiscussion(ctx, access.ID, discussionID, title, body, now); err != nil {
				return err
			}
			if err := tx.AppendActivity(ctx, workspaceactivity.Input{ProjectID: access.ID, ActorID: actorID, SubjectType: "discussion", SubjectID: discussionID, Type: "discussion.updated", At: now, Payload: map[string]any{"title_snapshot": title, "title_changed": title != row.Title, "body_changed": body != row.Body}}); err != nil {
				return err
			}
		}
		updated, err := tx.GetDiscussion(ctx, access.ID, discussionID, false)
		if err != nil {
			return err
		}
		mapped := mapDiscussion(*updated, actorID, ownerID(access), canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID)))
		response = &mapped
		return nil
	})
	if err != nil {
		return nil, mapDiscussionPersistenceError(err)
	}
	return response, nil
}

func (service *DiscussionService) Delete(ctx context.Context, actorID uuid.UUID, slug, discussionIDValue string) error {
	discussionID, err := parseDiscussionID(discussionIDValue)
	if err != nil {
		return err
	}
	access, err := service.resolveAccess(ctx, actorID, slug)
	if err != nil {
		return err
	}
	return mapDiscussionPersistenceError(service.repository.WithDiscussionTransaction(ctx, func(tx DiscussionTx) error {
		access, err = tx.ValidateDiscussionWriter(ctx, access.ID, actorID)
		if err != nil {
			return err
		}
		if !canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID)) {
			return ErrDiscussionProjectNotWritable
		}
		row, err := tx.GetDiscussion(ctx, access.ID, discussionID, true)
		if err != nil {
			return err
		}
		if !canDeleteDiscussion(row.AuthorID, actorID, ownerID(access)) {
			return sharedErrors.ErrForbidden
		}
		now := time.Now().UTC()
		if err := tx.SoftDeleteDiscussionReplies(ctx, access.ID, discussionID, now); err != nil {
			return err
		}
		if err := tx.SoftDeleteDiscussion(ctx, access.ID, discussionID, now); err != nil {
			return err
		}
		return tx.AppendActivity(ctx, workspaceactivity.Input{ProjectID: access.ID, ActorID: actorID, SubjectType: "discussion", SubjectID: discussionID, Type: "discussion.deleted", At: now, Payload: map[string]any{"author_user_id": row.AuthorID}})
	}))
}

func (service *DiscussionService) ListReplies(ctx context.Context, actorID uuid.UUID, slug, discussionIDValue string, requestedPage, requestedLimit int) ([]DiscussionReplyResponse, project.ProjectPaginationMeta, error) {
	access, err := service.resolveAccess(ctx, actorID, slug)
	if err != nil {
		return nil, project.ProjectPaginationMeta{}, err
	}
	discussionID, err := parseDiscussionID(discussionIDValue)
	if err != nil {
		return nil, project.ProjectPaginationMeta{}, err
	}
	if _, err := service.repository.GetDiscussion(ctx, access.ID, discussionID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, project.ProjectPaginationMeta{}, sharedErrors.ErrNotFound
		}
		return nil, project.ProjectPaginationMeta{}, fmt.Errorf("load workspace discussion before listing replies: %w", err)
	}
	page := normalizeDiscussionPage(requestedPage)
	limit := normalizeDiscussionPageSize(requestedLimit)
	rows, total, err := service.repository.ListDiscussionReplies(ctx, access.ID, discussionID, page, limit)
	if err != nil {
		return nil, project.ProjectPaginationMeta{}, fmt.Errorf("list workspace discussion replies: %w", err)
	}
	items := make([]DiscussionReplyResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapDiscussionReply(row, actorID, ownerID(access), canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID))))
	}
	return items, discussionPaginationMeta(page, limit, total), nil
}

func (service *DiscussionService) CreateReply(ctx context.Context, actorID uuid.UUID, slug, discussionIDValue, idempotencyKey string, request CreateDiscussionReplyRequest) (*DiscussionReplyResponse, bool, error) {
	discussionID, err := parseDiscussionID(discussionIDValue)
	if err != nil {
		return nil, false, err
	}
	body, err := normalizeDiscussionReplyBody(request.Body)
	if err != nil {
		return nil, false, err
	}
	key, err := normalizeDiscussionIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, false, err
	}
	access, err := service.resolveAccess(ctx, actorID, slug)
	if err != nil {
		return nil, false, err
	}
	hash := discussionRequestHash(body, normalizeParentID(request.ParentReplyID))
	var response *DiscussionReplyResponse
	created := false
	err = service.repository.WithDiscussionTransaction(ctx, func(tx DiscussionTx) error {
		access, err = tx.ValidateDiscussionWriter(ctx, access.ID, actorID)
		if err != nil {
			return err
		}
		_, err = tx.GetDiscussion(ctx, access.ID, discussionID, true)
		if err != nil {
			return err
		}
		existing, findErr := tx.FindReplyByIdempotency(ctx, access.ID, actorID, key)
		if findErr == nil {
			if existing.RequestHash == nil || *existing.RequestHash != hash {
				return ErrDiscussionIdempotencyConflict
			}
			if existing.DeletedAt != nil {
				return sharedErrors.ErrConflict
			}
			row, err := tx.GetDiscussionReply(ctx, access.ID, discussionID, existing.ID, false)
			if err != nil {
				return err
			}
			mapped := mapDiscussionReply(*row, actorID, ownerID(access), canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID)))
			response = &mapped
			return nil
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("find idempotent workspace reply: %w", findErr)
		}
		if !canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID)) {
			return ErrDiscussionProjectNotWritable
		}
		var parentID *uuid.UUID
		if request.ParentReplyID != nil && strings.TrimSpace(*request.ParentReplyID) != "" {
			parsed, err := parseDiscussionID(*request.ParentReplyID)
			if err != nil {
				return fmt.Errorf("parent_reply_id: %w", err)
			}
			parent, err := tx.GetDiscussionReply(ctx, access.ID, discussionID, parsed, true)
			if err != nil {
				return err
			}
			if parent.DeletedAt != nil {
				return ErrDiscussionReplyDeleted
			}
			if err := validateDiscussionParent(&parent.DiscussionReply); err != nil {
				return err
			}
			parentID = &parsed
		}
		now := time.Now().UTC()
		keyCopy, hashCopy := key, hash
		reply := &DiscussionReply{
			ID: uuid.New(), ProjectID: access.ID, DiscussionID: discussionID,
			ParentReplyID: parentID, AuthorID: actorID, Body: body,
			IdempotencyKey: &keyCopy, RequestHash: &hashCopy, CreatedAt: now, UpdatedAt: now,
		}
		inserted, err := tx.InsertDiscussionReply(ctx, reply)
		if err != nil {
			return fmt.Errorf("create workspace discussion reply: %w", err)
		}
		if !inserted {
			existing, err = tx.FindReplyByIdempotency(ctx, access.ID, actorID, key)
			if err != nil {
				return fmt.Errorf("resolve concurrent reply retry: %w", err)
			}
			if existing.RequestHash == nil || *existing.RequestHash != hash || existing.DeletedAt != nil {
				return ErrDiscussionIdempotencyConflict
			}
			reply.ID = existing.ID
		} else {
			created = true
			if err := tx.TouchDiscussion(ctx, access.ID, discussionID, now); err != nil {
				return err
			}
			if err := recordDiscussionReplyActivity(ctx, tx, access.ID, actorID, reply, "discussion.reply_created", now); err != nil {
				return err
			}
		}
		row, err := tx.GetDiscussionReply(ctx, access.ID, discussionID, reply.ID, false)
		if err != nil {
			return err
		}
		mapped := mapDiscussionReply(*row, actorID, ownerID(access), canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID)))
		response = &mapped
		return nil
	})
	if err != nil {
		return nil, false, mapDiscussionPersistenceError(err)
	}
	return response, created, nil
}

func (service *DiscussionService) UpdateReply(ctx context.Context, actorID uuid.UUID, slug, discussionIDValue, replyIDValue string, request UpdateDiscussionReplyRequest) (*DiscussionReplyResponse, error) {
	discussionID, err := parseDiscussionID(discussionIDValue)
	if err != nil {
		return nil, err
	}
	replyID, err := parseDiscussionID(replyIDValue)
	if err != nil {
		return nil, err
	}
	body, err := normalizeDiscussionReplyBody(request.Body)
	if err != nil {
		return nil, err
	}
	access, err := service.resolveAccess(ctx, actorID, slug)
	if err != nil {
		return nil, err
	}
	var response *DiscussionReplyResponse
	err = service.repository.WithDiscussionTransaction(ctx, func(tx DiscussionTx) error {
		access, err = tx.ValidateDiscussionWriter(ctx, access.ID, actorID)
		if err != nil {
			return err
		}
		if !canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID)) {
			return ErrDiscussionProjectNotWritable
		}
		row, err := tx.GetDiscussionReply(ctx, access.ID, discussionID, replyID, true)
		if err != nil {
			return err
		}
		if row.DeletedAt != nil {
			return sharedErrors.ErrNotFound
		}
		if !canEditDiscussion(row.AuthorID, actorID) {
			return sharedErrors.ErrForbidden
		}
		now := time.Now().UTC()
		if body != row.Body {
			if err := tx.UpdateDiscussionReply(ctx, access.ID, discussionID, replyID, body, now); err != nil {
				return err
			}
			if err := tx.TouchDiscussion(ctx, access.ID, discussionID, now); err != nil {
				return err
			}
			if err := recordDiscussionReplyActivity(ctx, tx, access.ID, actorID, &row.DiscussionReply, "discussion.reply_updated", now); err != nil {
				return err
			}
		}
		updated, err := tx.GetDiscussionReply(ctx, access.ID, discussionID, replyID, false)
		if err != nil {
			return err
		}
		mapped := mapDiscussionReply(*updated, actorID, ownerID(access), canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID)))
		response = &mapped
		return nil
	})
	if err != nil {
		return nil, mapDiscussionPersistenceError(err)
	}
	return response, nil
}

func (service *DiscussionService) DeleteReply(ctx context.Context, actorID uuid.UUID, slug, discussionIDValue, replyIDValue string) error {
	discussionID, err := parseDiscussionID(discussionIDValue)
	if err != nil {
		return err
	}
	replyID, err := parseDiscussionID(replyIDValue)
	if err != nil {
		return err
	}
	access, err := service.resolveAccess(ctx, actorID, slug)
	if err != nil {
		return err
	}
	return mapDiscussionPersistenceError(service.repository.WithDiscussionTransaction(ctx, func(tx DiscussionTx) error {
		access, err = tx.ValidateDiscussionWriter(ctx, access.ID, actorID)
		if err != nil {
			return err
		}
		if !canWriteWorkspaceDiscussions(access.Status, access.IsActiveMember, access.IsOwner(actorID)) {
			return ErrDiscussionProjectNotWritable
		}
		row, err := tx.GetDiscussionReply(ctx, access.ID, discussionID, replyID, true)
		if err != nil {
			return err
		}
		if row.DeletedAt != nil {
			return sharedErrors.ErrNotFound
		}
		if !canDeleteDiscussion(row.AuthorID, actorID, ownerID(access)) {
			return sharedErrors.ErrForbidden
		}
		now := time.Now().UTC()
		if err := tx.SoftDeleteDiscussionReply(ctx, access.ID, discussionID, replyID, now); err != nil {
			return err
		}
		if err := tx.TouchDiscussion(ctx, access.ID, discussionID, now); err != nil {
			return err
		}
		return recordDiscussionReplyActivity(ctx, tx, access.ID, actorID, &row.DiscussionReply, "discussion.reply_deleted", now)
	}))
}

func (service *DiscussionService) resolveAccess(ctx context.Context, actorID uuid.UUID, slug string) (*DiscussionProjectAccess, error) {
	if actorID == uuid.Nil || strings.TrimSpace(slug) == "" {
		return nil, sharedErrors.ErrBadRequest
	}
	access, err := service.repository.ResolveDiscussionProject(ctx, slug, actorID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, sharedErrors.ErrNotFound
		}
		return nil, fmt.Errorf("resolve workspace discussion project: %w", err)
	}
	if access == nil || !canReadWorkspaceDiscussions(access.IsActiveMember, access.IsOwner(actorID)) {
		return nil, sharedErrors.ErrNotFound
	}
	return access, nil
}

func normalizeDiscussionPageSize(limit int) int {
	if limit < 1 {
		return DiscussionDefaultPageSize
	}
	if limit > DiscussionMaxPageSize {
		return DiscussionMaxPageSize
	}
	return limit
}

func normalizeDiscussionPage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func discussionPaginationMeta(page, limit int, total int64) project.ProjectPaginationMeta {
	pages := 0
	if total > 0 {
		pages = int((total + int64(limit) - 1) / int64(limit))
	}
	return project.ProjectPaginationMeta{Page: page, Limit: limit, Total: total, TotalPages: pages}
}

func normalizeDiscussionIdempotencyKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 100 || strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("%w: Idempotency-Key must contain 1 to 100 characters", sharedErrors.ErrBadRequest)
	}
	return value, nil
}

func parseDiscussionID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, fmt.Errorf("%w: discussion resource ID must be a UUID", sharedErrors.ErrBadRequest)
	}
	return id, nil
}

func discussionRequestHash(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}

func normalizeParentID(value *string) string {
	if value == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(*value))
}

func ownerID(access *DiscussionProjectAccess) uuid.UUID {
	if access.OwnerID == nil {
		return uuid.Nil
	}
	return *access.OwnerID
}

func mapDiscussionPersistenceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return sharedErrors.ErrNotFound
	}
	return err
}

func bodyPreview(body string) string {
	runes := []rune(body)
	if len(runes) <= 240 {
		return body
	}
	return string(runes[:240]) + "…"
}

func mapDiscussionAuthor(userID uuid.UUID, name string, avatar *string) DiscussionAuthorResponse {
	if strings.TrimSpace(name) == "" {
		name = "Anggota"
	}
	return DiscussionAuthorResponse{ID: userID.String(), Name: name, AvatarURL: avatar}
}

func mapDiscussion(row DiscussionProjection, actorID, owner uuid.UUID, canWrite bool) DiscussionResponse {
	return DiscussionResponse{
		ID: row.ID.String(), ProjectID: row.ProjectID.String(), ProjectTitle: row.ProjectTitle, Title: row.Title, Body: row.Body,
		Author:     mapDiscussionAuthor(row.AuthorID, row.AuthorName, row.AuthorAvatar),
		ReplyCount: row.ReplyCount, ParticipantCount: row.ParticipantCount,
		LastActivityAt: row.LastActivityAt.UTC().Format(time.RFC3339Nano),
		CreatedAt:      row.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339Nano),
		CanEdit: canWrite && canEditDiscussion(row.AuthorID, actorID), CanDelete: canWrite && canDeleteDiscussion(row.AuthorID, actorID, owner),
	}
}

func mapDiscussionListItem(row DiscussionProjection, actorID, owner uuid.UUID, canWrite bool) DiscussionListItemResponse {
	discussion := mapDiscussion(row, actorID, owner, canWrite)
	return DiscussionListItemResponse{
		ID: discussion.ID, Title: discussion.Title, BodyPreview: bodyPreview(discussion.Body), Author: discussion.Author,
		ReplyCount: discussion.ReplyCount, ParticipantCount: discussion.ParticipantCount,
		LastActivityAt: discussion.LastActivityAt, CreatedAt: discussion.CreatedAt, UpdatedAt: discussion.UpdatedAt,
		CanEdit: discussion.CanEdit, CanDelete: discussion.CanDelete,
	}
}

func mapDiscussionReply(row DiscussionReplyProjection, actorID, owner uuid.UUID, canWrite bool) DiscussionReplyResponse {
	var parentID *string
	if row.ParentReplyID != nil {
		value := row.ParentReplyID.String()
		parentID = &value
	}
	children := make([]DiscussionReplyResponse, 0, len(row.Replies))
	for _, child := range row.Replies {
		children = append(children, mapDiscussionReply(child, actorID, owner, canWrite))
	}
	body := row.Body
	if row.DeletedAt != nil {
		body = ""
	}
	return DiscussionReplyResponse{
		ID: row.ID.String(), DiscussionID: row.DiscussionID.String(), ParentReplyID: parentID, Body: body,
		Author:    mapDiscussionAuthor(row.AuthorID, row.AuthorName, row.AuthorAvatar),
		CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339Nano),
		IsDeleted: row.DeletedAt != nil, CanEdit: canWrite && row.DeletedAt == nil && canEditDiscussion(row.AuthorID, actorID),
		CanDelete: canWrite && row.DeletedAt == nil && canDeleteDiscussion(row.AuthorID, actorID, owner), Replies: children,
	}
}

func discussionRuneLength(value string) int { return utf8.RuneCountInString(value) }

func recordDiscussionReplyActivity(ctx context.Context, tx DiscussionTx, projectID, actorID uuid.UUID, reply *DiscussionReply, eventType string, now time.Time) error {
	topic, err := tx.GetDiscussion(ctx, projectID, reply.DiscussionID, false)
	if err != nil {
		return err
	}
	return tx.AppendActivity(ctx, workspaceactivity.Input{ProjectID: projectID, ActorID: actorID,
		SubjectType: "discussion_reply", SubjectID: reply.ID, Type: eventType, At: now,
		Payload: map[string]any{"discussion_id": reply.DiscussionID, "parent_reply_id": reply.ParentReplyID, "author_user_id": reply.AuthorID, "title_snapshot": topic.Title},
	})
}
