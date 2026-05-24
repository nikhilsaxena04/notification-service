package router

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	pb "notification-service/proto/notificationpb"
)

// Provider interface abstracts a downstream messaging provider.
type Provider interface {
	Send(ctx context.Context, req *pb.DispatchRequest) (providerID string, err error)
}

// MockEmailProvider simulates an email provider like Mailtrap.
type MockEmailProvider struct {
	logger *slog.Logger
}

func NewMockEmailProvider(logger *slog.Logger) *MockEmailProvider {
	return &MockEmailProvider{logger: logger}
}

func (p *MockEmailProvider) Send(ctx context.Context, req *pb.DispatchRequest) (string, error) {
	email := req.GetEmail()
	providerID := "email-" + uuid.New().String()
	p.logger.Info("MockEmailProvider: Simulated sending email", "to", email.To, "subject", email.Subject, "provider_id", providerID)
	return providerID, nil
}

// MockSMSProvider simulates an SMS provider like Twilio.
type MockSMSProvider struct {
	logger *slog.Logger
}

func NewMockSMSProvider(logger *slog.Logger) *MockSMSProvider {
	return &MockSMSProvider{logger: logger}
}

func (p *MockSMSProvider) Send(ctx context.Context, req *pb.DispatchRequest) (string, error) {
	sms := req.GetSms()
	providerID := "sms-" + uuid.New().String()
	p.logger.Info("MockSMSProvider: Simulated sending SMS", "to", sms.To, "provider_id", providerID)
	return providerID, nil
}

// MockPushProvider simulates a push notification provider like FCM.
type MockPushProvider struct {
	logger *slog.Logger
}

func NewMockPushProvider(logger *slog.Logger) *MockPushProvider {
	return &MockPushProvider{logger: logger}
}

func (p *MockPushProvider) Send(ctx context.Context, req *pb.DispatchRequest) (string, error) {
	push := req.GetPush()
	providerID := "push-" + uuid.New().String()
	p.logger.Info("MockPushProvider: Simulated sending push", "device_token", push.DeviceToken, "title", push.Title, "provider_id", providerID)
	return providerID, nil
}
