package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	GenerationTestMaxQuestions = 20
	GenerationTestMaxRecords   = 1000
	GenerationTestLibraryLimit = 500
)

func validGenerationKind(kind string) bool {
	return kind == TestKindPelican || kind == TestKindIntelligence
}

func validateGenerationIDs(ids []int64, limit int) error {
	if len(ids) == 0 || len(ids) > limit {
		return fmt.Errorf("%w: select 1 to %d items", ErrInvalidPelicanBatch, limit)
	}
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return fmt.Errorf("%w: IDs must be positive and unique", ErrInvalidPelicanBatch)
		}
		seen[id] = true
	}
	return nil
}

func (s *PelicanTestService) ListQuestions(ctx context.Context, kind string) ([]GenerationTestQuestion, error) {
	if !validGenerationKind(kind) {
		return nil, ErrInvalidPelicanBatch
	}
	return s.repo.ListQuestions(ctx, kind)
}

func (s *PelicanTestService) SaveQuestion(ctx context.Context, question GenerationTestQuestion) (*GenerationTestQuestion, error) {
	question.Name = strings.TrimSpace(question.Name)
	question.Prompt = strings.TrimSpace(question.Prompt)
	question.ExpectedAnswer = strings.TrimSpace(question.ExpectedAnswer)
	if !validGenerationKind(question.TestKind) || question.ID < 0 || question.Name == "" || utf8.RuneCountInString(question.Name) > 120 || question.Prompt == "" || utf8.RuneCountInString(question.Prompt) > PelicanTestMaxPrompt {
		return nil, fmt.Errorf("%w: name (1–120) and prompt (1–4000) are required", ErrInvalidPelicanBatch)
	}
	if question.TestKind == TestKindIntelligence {
		question.ExpectedAnswer = parseIntelligenceAnswer(question.ExpectedAnswer)
		if question.ExpectedAnswer == "" {
			return nil, fmt.Errorf("%w: a numeric expected answer is required", ErrInvalidPelicanBatch)
		}
	} else {
		question.ExpectedAnswer = ""
	}
	return s.repo.SaveQuestion(ctx, question)
}

func (s *PelicanTestService) GetTestPreference(ctx context.Context, userID int64, kind string) (*GenerationTestPreference, error) {
	if !validGenerationKind(kind) || userID <= 0 {
		return nil, ErrInvalidPelicanBatch
	}
	return s.repo.GetTestPreference(ctx, userID, kind)
}

// The repository resolves and locks the questions, then saves snapshots and
// the actor's preference in one transaction. Failed submissions never replace it.
func (s *PelicanTestService) StartQuestionBatch(ctx context.Context, accountIDs []int64, modelID, kind string, questionIDs []int64, userID int64) ([]PelicanTest, error) {
	if !validGenerationKind(kind) || userID <= 0 {
		return nil, ErrInvalidPelicanBatch
	}
	if err := validateGenerationIDs(accountIDs, PelicanTestMaxAccounts); err != nil {
		return nil, err
	}
	if err := validateGenerationIDs(questionIDs, GenerationTestMaxQuestions); err != nil {
		return nil, err
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" || len(modelID) > 255 || len(accountIDs)*len(questionIDs) > GenerationTestMaxRecords || isImageGenerationModel(modelID) || isOpenAIImageModel(modelID) || isGrokVideoGenerationModel(modelID) {
		return nil, fmt.Errorf("%w: select a text model and at most %d records", ErrInvalidPelicanBatch, GenerationTestMaxRecords)
	}
	if err := s.repo.ExpireStale(ctx); err != nil {
		return nil, err
	}
	return s.repo.CreateQuestionBatch(ctx, uuid.NewString(), accountIDs, modelID, kind, questionIDs, userID)
}

func (s *PelicanTestService) ListLatestBatches(ctx context.Context, accountIDs []int64, kind, batchID string) ([]PelicanTest, error) {
	if !validGenerationKind(kind) || len(batchID) > 64 {
		return nil, ErrInvalidPelicanBatch
	}
	if len(accountIDs) == 0 {
		return []PelicanTest{}, nil
	}
	if err := validateGenerationIDs(accountIDs, PelicanTestMaxAccounts); err != nil {
		return nil, err
	}
	return s.repo.ListLatestBatches(ctx, accountIDs, kind, batchID)
}

func (s *PelicanTestService) ListBatchHistory(ctx context.Context, accountID int64, page, pageSize int, kind string) (*PelicanTestHistory, error) {
	if !validGenerationKind(kind) || accountID <= 0 || page < 1 || page > 1000000 || pageSize < 1 || pageSize > 50 {
		return nil, ErrInvalidPelicanBatch
	}
	return s.repo.ListBatchHistory(ctx, accountID, page, pageSize, kind)
}
