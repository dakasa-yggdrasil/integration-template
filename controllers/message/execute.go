package message

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dakasa-co/yggdrasil-integration-template/internal/adapter"
	"github.com/dakasa-co/yggdrasil-integration-template/internal/protocol"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

func executeHandler(conn *amqp.Connection, logger *zap.Logger) ConsumerHandler {
	return func(ctx context.Context, d amqp.Delivery) error {
		var envelope struct {
			Operation  string `json:"operation"`
			Capability string `json:"capability,omitempty"`
		}
		if err := json.Unmarshal(d.Body, &envelope); err != nil {
			return replyFailure(ctx, conn, d, "bad_request", err, logger)
		}

		operation := adapter.NormalizeExecuteOperation(envelope.Operation)

		capability := adapter.NormalizeExecuteCapability(envelope.Capability)
		if !adapter.SupportsExecuteCapability(capability) {
			return replyFailure(ctx, conn, d, "unsupported_capability", fmt.Errorf("unsupported capability %q", envelope.Capability), logger)
		}

		switch operation {
		case adapter.OperationGenerateInstallation:
			var req protocol.AdapterGenerateInstallationRequest
			if err := json.Unmarshal(d.Body, &req); err != nil {
				return replyFailure(ctx, conn, d, "bad_request", err, logger)
			}
			response, err := adapter.GenerateInstallation(req)
			if err != nil {
				return replyFailure(ctx, conn, d, "generation_failed", err, logger)
			}
			return replySuccess(ctx, conn, d, response, logger)
		case adapter.OperationReconcileInstallation:
			var req protocol.AdapterReconcileInstallationRequest
			if err := json.Unmarshal(d.Body, &req); err != nil {
				return replyFailure(ctx, conn, d, "bad_request", err, logger)
			}
			response, err := adapter.ReconcileInstallation(req)
			if err != nil {
				return replyFailure(ctx, conn, d, "reconcile_failed", err, logger)
			}
			return replySuccess(ctx, conn, d, response, logger)
		case adapter.OperationDiscoverInstallationState:
			var req protocol.AdapterDiscoverInstallationStateRequest
			if err := json.Unmarshal(d.Body, &req); err != nil {
				return replyFailure(ctx, conn, d, "bad_request", err, logger)
			}
			response, err := adapter.DiscoverInstallationState(req)
			if err != nil {
				return replyFailure(ctx, conn, d, "discover_failed", err, logger)
			}
			return replySuccess(ctx, conn, d, response, logger)
		default:
			return replyFailure(ctx, conn, d, "unsupported_operation", fmt.Errorf("unsupported operation %q", envelope.Operation), logger)
		}
	}
}
