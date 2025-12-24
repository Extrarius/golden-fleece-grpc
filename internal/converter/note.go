package converter

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	"notes-service/internal/model"
	notesv1 "notes-service/pkg/proto/notes/v1"
)

// ProtoToModel конвертирует proto Note в domain модель
func ProtoToModel(protoNote *notesv1.Note) model.Note {
	if protoNote == nil {
		return model.Note{}
	}

	var createdAt, updatedAt time.Time
	if protoNote.GetCreatedAt() != nil {
		createdAt = protoNote.GetCreatedAt().AsTime()
	}
	if protoNote.GetUpdatedAt() != nil {
		updatedAt = protoNote.GetUpdatedAt().AsTime()
	}

	return model.Note{
		ID:        protoNote.GetId(),
		Title:     protoNote.GetTitle(),
		Content:   protoNote.GetContent(),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

// ModelToProto конвертирует domain модель Note в proto
func ModelToProto(note model.Note) *notesv1.Note {
	var createdAt, updatedAt *timestamppb.Timestamp
	if !note.CreatedAt.IsZero() {
		createdAt = timestamppb.New(note.CreatedAt)
	}
	if !note.UpdatedAt.IsZero() {
		updatedAt = timestamppb.New(note.UpdatedAt)
	}

	return &notesv1.Note{
		Id:        note.ID,
		Title:     note.Title,
		Content:   note.Content,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

// ModelsToProtos конвертирует слайс domain моделей в слайс proto
func ModelsToProtos(notes []model.Note) []*notesv1.Note {
	if notes == nil {
		return nil
	}

	protoNotes := make([]*notesv1.Note, len(notes))
	for i, note := range notes {
		protoNotes[i] = ModelToProto(note)
	}

	return protoNotes
}
