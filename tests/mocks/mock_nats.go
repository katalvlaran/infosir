package mocks

type NatsPublishScenario struct {
	Subject     string
	ReturnError error
}

type MockNatsClient struct {
	// Можем хранить сценарии, как в случае с Binance
	PublishScenarios []NatsPublishScenario

	PublishCalls []struct {
		Subject string
		Data    []byte
	}
}

func NewMockNatsClient(scenarios []NatsPublishScenario) *MockNatsClient {
	return &MockNatsClient{PublishScenarios: scenarios}
}

func (m *MockNatsClient) PublishJS(subject string, data []byte) error {
	// Логируем вызов
	m.PublishCalls = append(m.PublishCalls, struct {
		Subject string
		Data    []byte
	}{subject, data})

	// Ищем подходящий сценарий
	for _, sc := range m.PublishScenarios {
		if sc.Subject == subject {
			return sc.ReturnError
		}
	}

	// Если нет подходящего сценария, возвращаем nil (по умолчанию)
	return nil
}

/*
scenarios := []mocks.NatsPublishScenario{
    {Subject: "infosir_kline", ReturnError: nil}, // ok
    {Subject: "some_other_subject", ReturnError: fmt.Errorf("JS down")},
}
mockNats := mocks.NewMockNatsClient(scenarios)

// ... pass this mock to your service ...

*/
