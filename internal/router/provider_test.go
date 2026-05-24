package router

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	pb "notification-service/proto/notificationpb"
)

func TestHandler_Dispatch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	
	email := NewMockEmailProvider(logger)
	sms := NewMockSMSProvider(logger)
	push := NewMockPushProvider(logger)
	
	handler := NewHandler(email, sms, push, logger)

	tests := []struct {
		name          string
		req           *pb.DispatchRequest
		wantSuccess   bool
		wantErrSubstr string
	}{
		{
			name: "Valid Email",
			req: &pb.DispatchRequest{
				JobId:   "123",
				Channel: pb.Channel_CHANNEL_EMAIL,
				Payload: &pb.DispatchRequest_Email{
					Email: &pb.EmailPayload{To: []string{"test@example.com"}},
				},
			},
			wantSuccess: true,
		},
		{
			name: "Missing Email Payload",
			req: &pb.DispatchRequest{
				JobId:   "123",
				Channel: pb.Channel_CHANNEL_EMAIL,
			},
			wantSuccess:   false,
			wantErrSubstr: "missing email payload",
		},
		{
			name: "Valid SMS",
			req: &pb.DispatchRequest{
				JobId:   "123",
				Channel: pb.Channel_CHANNEL_SMS,
				Payload: &pb.DispatchRequest_Sms{
					Sms: &pb.SMSPayload{To: "1234567890"},
				},
			},
			wantSuccess: true,
		},
		{
			name: "Valid Push",
			req: &pb.DispatchRequest{
				JobId:   "123",
				Channel: pb.Channel_CHANNEL_PUSH,
				Payload: &pb.DispatchRequest_Push{
					Push: &pb.PushPayload{DeviceToken: "token_123"},
				},
			},
			wantSuccess: true,
		},
		{
			name: "Unsupported Channel",
			req: &pb.DispatchRequest{
				JobId:   "123",
				Channel: pb.Channel_CHANNEL_UNSPECIFIED,
			},
			wantSuccess:   false,
			wantErrSubstr: "unsupported channel",
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handler.Dispatch(ctx, tt.req)
			if err != nil {
				t.Fatalf("Unexpected gRPC error: %v", err)
			}
			
			if resp.Success != tt.wantSuccess {
				t.Errorf("Expected success=%v, got %v", tt.wantSuccess, resp.Success)
			}
			
			if !tt.wantSuccess && tt.wantErrSubstr != "" {
				if !strings.Contains(resp.ErrorMessage, tt.wantErrSubstr) {
					t.Errorf("Expected error containing %q, got %q", tt.wantErrSubstr, resp.ErrorMessage)
				}
			}
			
			if tt.wantSuccess && resp.ProviderId == "" {
				t.Error("Expected provider ID to be populated on success")
			}
		})
	}
}
