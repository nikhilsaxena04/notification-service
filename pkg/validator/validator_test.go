package validator

import (
	"testing"

	"github.com/google/uuid"
	pb "notification-service/proto/notificationpb"
)

func TestValidateSendRequest(t *testing.T) {
	validUUID := uuid.New().String()

	tests := []struct {
		name    string
		req     *pb.SendNotificationRequest
		wantErr error
	}{
		{
			name: "Valid Email Request",
			req: &pb.SendNotificationRequest{
				IdempotencyKey: validUUID,
				Channel:        pb.Channel_CHANNEL_EMAIL,
				Payload: &pb.SendNotificationRequest_Email{
					Email: &pb.EmailPayload{
						From:     "test@example.com",
						To:       []string{"user@example.com"},
						Subject:  "Test",
						BodyHtml: "<p>test</p>",
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "Invalid Idempotency Key",
			req: &pb.SendNotificationRequest{
				IdempotencyKey: "not-a-uuid",
				Channel:        pb.Channel_CHANNEL_EMAIL,
				Payload: &pb.SendNotificationRequest_Email{
					Email: &pb.EmailPayload{
						From:     "test@example.com",
						To:       []string{"user@example.com"},
						Subject:  "Test",
						BodyHtml: "<p>test</p>",
					},
				},
			},
			wantErr: ErrInvalidIdempotencyKey,
		},
		{
			name: "Unspecified Channel",
			req: &pb.SendNotificationRequest{
				IdempotencyKey: validUUID,
				Channel:        pb.Channel_CHANNEL_UNSPECIFIED,
			},
			wantErr: ErrInvalidChannel,
		},
		{
			name: "Payload Mismatch",
			req: &pb.SendNotificationRequest{
				IdempotencyKey: validUUID,
				Channel:        pb.Channel_CHANNEL_EMAIL,
				Payload: &pb.SendNotificationRequest_Sms{
					Sms: &pb.SMSPayload{},
				},
			},
			wantErr: ErrPayloadMismatch,
		},
		{
			name: "Missing Email Fields",
			req: &pb.SendNotificationRequest{
				IdempotencyKey: validUUID,
				Channel:        pb.Channel_CHANNEL_EMAIL,
				Payload: &pb.SendNotificationRequest_Email{
					Email: &pb.EmailPayload{
						From: "test@example.com",
						// Missing To, Subject, BodyHtml
					},
				},
			},
			wantErr: ErrMissingEmailFields,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSendRequest(tt.req)
			if err != tt.wantErr {
				t.Errorf("ValidateSendRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
