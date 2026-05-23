package validator

import (
	"errors"

	"github.com/google/uuid"
	pb "notification-service/proto/notificationpb"
)

var (
	ErrInvalidChannel      = errors.New("channel is UNSPECIFIED or invalid")
	ErrPayloadMismatch     = errors.New("payload does not match channel")
	ErrMissingEmailFields  = errors.New("email payload missing required fields (from, to, subject, body_html)")
	ErrMissingSMSFields    = errors.New("sms payload missing required fields (from, to, body)")
	ErrMissingPushFields   = errors.New("push payload missing required fields (device_token, title, body)")
	ErrInvalidIdempotencyKey = errors.New("idempotency_key must be a valid UUIDv4")
)

// ValidateSendRequest validates an incoming SendNotificationRequest.
func ValidateSendRequest(req *pb.SendNotificationRequest) error {
	if _, err := uuid.Parse(req.IdempotencyKey); err != nil {
		return ErrInvalidIdempotencyKey
	}

	if req.Channel == pb.Channel_CHANNEL_UNSPECIFIED {
		return ErrInvalidChannel
	}

	switch req.Channel {
	case pb.Channel_CHANNEL_EMAIL:
		email := req.GetEmail()
		if email == nil {
			return ErrPayloadMismatch
		}
		if email.From == "" || len(email.To) == 0 || email.Subject == "" || email.BodyHtml == "" {
			return ErrMissingEmailFields
		}
	case pb.Channel_CHANNEL_SMS:
		sms := req.GetSms()
		if sms == nil {
			return ErrPayloadMismatch
		}
		if sms.From == "" || sms.To == "" || sms.Body == "" {
			return ErrMissingSMSFields
		}
	case pb.Channel_CHANNEL_PUSH:
		push := req.GetPush()
		if push == nil {
			return ErrPayloadMismatch
		}
		if push.DeviceToken == "" || push.Title == "" || push.Body == "" {
			return ErrMissingPushFields
		}
	default:
		return ErrInvalidChannel
	}

	return nil
}
