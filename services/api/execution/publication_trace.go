package execution

import (
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// PublicationTrace intentionally cannot contain payloads, URLs, credentials,
// ciphertext, SDK error messages, or cancellation reason messages/items.
// Phase identifies the attempted boundary; Code=ok acknowledges that boundary.
type PublicationTrace struct {
	Phase       string `json:"phase"`
	Code        string `json:"code"`
	ProjectID   string `json:"projectId"`
	RunID       string `json:"runId,omitempty"`
	JobID       string `json:"jobId"`
	ExecutionID string `json:"executionId"`
	LeaseID     string `json:"leaseId,omitempty"`
	Fence       int64  `json:"fence"`
	TicketID    string `json:"ticketId,omitempty"`
}

func (s *Service) tracePublication(job, ticket row, phase string, err error) {
	if s.PublicationTrace == nil {
		return
	}
	for _, value := range []string{job.ProjectID, job.JobID, job.ExecutionID} {
		if !uuidPattern.MatchString(value) {
			return
		}
	}
	for _, value := range []string{job.RunID, job.LeaseID} {
		if value != "" && !uuidPattern.MatchString(value) {
			return
		}
	}
	switch phase {
	case "transaction_conflict", "claimed", "intent", "register", "registered", "writing", "put_ack", "settle", "settled", "finalize":
	default:
		return
	}
	ticketID := strings.TrimPrefix(ticket.SK, "UPLOAD#")
	if ticketID != "" && !uuidPattern.MatchString(ticketID) {
		return
	}
	code := "ok"
	if err != nil {
		code = "publication_unavailable"
		if transactionConflict(err) {
			code = "transaction_conflict"
		} else {
			var canceled *dt.TransactionCanceledException
			if errors.As(err, &canceled) {
				code = "transaction_canceled"
				for _, reason := range canceled.CancellationReasons {
					if aws.ToString(reason.Code) == "ConditionalCheckFailed" {
						code = "condition_failed"
					}
				}
			}
		}
	}
	s.PublicationTrace(PublicationTrace{Phase: phase, Code: code, ProjectID: job.ProjectID, RunID: job.RunID, JobID: job.JobID, ExecutionID: job.ExecutionID, LeaseID: job.LeaseID, Fence: job.Fence, TicketID: ticketID})
}
