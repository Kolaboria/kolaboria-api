package workspace

import (
	"encoding/json"
	"strings"
)

// Administrative facts remain persisted but are not part of the member feed.
var activityDescriptions = map[string]string{
	"task.created":             "membuat task %s",
	"task.content_updated":     "memperbarui isi task %s",
	"task.metadata_updated":    "memperbarui prioritas atau target selesai task %s",
	"task.assignment_changed":  "mengubah penanggung jawab task %s",
	"task.status_changed":      "mengubah status task %s",
	"task.deleted":             "menghapus task %s",
	"task.comment_created":     "menambahkan komentar pada task %s",
	"discussion.created":       "memulai diskusi %s",
	"discussion.updated":       "memperbarui diskusi %s",
	"discussion.deleted":       "menghapus diskusi %s",
	"discussion.reply_created": "membalas diskusi %s",
	"discussion.reply_updated": "memperbarui balasan pada diskusi %s",
	"discussion.reply_deleted": "menghapus balasan pada diskusi %s",
	"project.started":          "memulai proyek",
	"project.archived":         "mengarsipkan proyek",
	"project.completed":        "menyelesaikan proyek setelah meninjau hasil dan scope task",
	"member.joined":            "menerima %s sebagai anggota proyek",
}

func visibleActivityTypes() []string {
	types := make([]string, 0, len(activityDescriptions))
	for name := range activityDescriptions {
		types = append(types, name)
	}
	return types
}

type activityPayload struct {
	From        string `json:"from"`
	To          string `json:"to"`
	CloseReason string `json:"close_reason"`
}

func mapActivity(row ActivityProjection) ActivityResponse {
	name := strings.TrimSpace(row.ActorName)
	if name == "" {
		name = "Pengguna tidak tersedia"
	}
	label := strings.TrimSpace(row.SubjectLabel)
	// Older events may not reference a content revision. Identify their task
	// using its current title without overriding an available historical label.
	if row.TaskID != nil && label == "" {
		label = strings.TrimSpace(row.CurrentTaskTitle)
		if label == "" {
			label = "Task #" + row.TaskID.String()[:8]
		}
	}
	availability := "available"
	if !row.SubjectAvailable {
		availability = "unavailable"
		label = "sumber yang tidak tersedia"
	}
	if row.SubjectDeleted {
		availability = "deleted"
		label = "sumber yang telah dihapus"
	}
	if label == "" {
		label = "sumber terkait"
	}
	quoted := "“" + label + "”"
	template, known := activityDescriptions[row.EventType]
	if !known {
		template = "memperbarui aktivitas proyek"
	}
	description := strings.ReplaceAll(template, "%s", quoted)
	item := ActivityResponse{ID: row.ID, EventType: row.EventType, OccurredAt: row.OccurredAt.UTC(),
		Actor:   ActivityActorResponse{UserID: row.ActorUserID, MembershipID: row.ActorMemberID, DisplayName: name, Avatar: row.ActorAvatar},
		Subject: ActivitySubjectResponse{Type: row.SubjectType, ID: row.SubjectID, Label: label, Availability: availability}, Description: description}
	if row.EventType != "task.status_changed" {
		return item
	}
	var payload activityPayload
	if json.Unmarshal(row.Payload, &payload) != nil {
		return item
	}
	item.Changes = &ActivityChangesResponse{FromStatus: payload.From, ToStatus: payload.To}
	switch {
	case payload.From == "review" && payload.To == "done":
		item.Description = "menerima hasil review task " + quoted
		item.Changes.SelfReview = row.SelfReview
	case payload.To == "review":
		item.Description = "mengirim task " + quoted + " untuk review"
	case payload.From == "review" && payload.To == "in_progress":
		item.Description = "mengembalikan task " + quoted + " untuk dikerjakan kembali"
		item.Changes.SelfReview = row.SelfReview
	case payload.To == "closed":
		item.Description = "menutup task " + quoted + " karena perubahan scope"
		if availability == "available" && strings.TrimSpace(payload.CloseReason) != "" {
			item.Description += ": " + runePrefix(payload.CloseReason, 240)
		}
	case payload.From == "done" || payload.From == "closed":
		item.Description = "membuka kembali task " + quoted
	case payload.To == "in_progress":
		item.Description = "mulai mengerjakan task " + quoted
	}
	if item.Changes.SelfReview != nil && *item.Changes.SelfReview {
		item.Description += " (self-review)"
	}
	return item
}
