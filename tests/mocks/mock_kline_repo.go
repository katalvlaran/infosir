// tests/mocks/mock_kline_repo.go

package mocks

import (
	"context"
	"fmt"

	"infosir/internal/model"
)

type FindLastScenario struct {
	TableName   string
	ReturnKline model.Kline
	ReturnError error
}

type BatchInsertScenario struct {
	TableName    string
	KlinesToFail bool // or ReturnError error
}

type MockKlineRepository struct {
	FindLastScenarios    []FindLastScenario
	BatchInsertScenarios []BatchInsertScenario

	FindLastCalls    []string // track tableName
	BatchInsertCalls []struct {
		TableName string
		Count     int
	}
}

func NewMockKlineRepository(flScn []FindLastScenario, biScn []BatchInsertScenario) *MockKlineRepository {
	return &MockKlineRepository{
		FindLastScenarios:    flScn,
		BatchInsertScenarios: biScn,
	}
}

func (m *MockKlineRepository) FindLast(ctx context.Context, tableName string) (model.Kline, error) {
	// log
	m.FindLastCalls = append(m.FindLastCalls, tableName)
	// find scenario
	for _, sc := range m.FindLastScenarios {
		if sc.TableName == tableName {
			return sc.ReturnKline, sc.ReturnError
		}
	}
	return model.Kline{}, fmt.Errorf("no scenario found for table=%s", tableName)
}

func (m *MockKlineRepository) BatchInsertKline(ctx context.Context, tableName string, klines []model.Kline) error {
	m.BatchInsertCalls = append(m.BatchInsertCalls, struct {
		TableName string
		Count     int
	}{tableName, len(klines)})

	// find scenario
	for _, sc := range m.BatchInsertScenarios {
		if sc.TableName == tableName {
			// if sc.KlinesToFail => return error
			if sc.KlinesToFail {
				return fmt.Errorf("forced error in BatchInsert for table %s", tableName)
			}
			// else success
		}
	}
	return nil
}

/*
// optionally implement other methods if needed
func (m *MockKlineRepository) FindMany(...) ([]model.Kline, error) {
	// ...
	return nil, nil
}
*/
