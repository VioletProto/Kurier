// Package ownership implements only the local users/projects slice.
package ownership

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"time"
)

type User struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	CreatedAt   string `json:"createdAt"`
}

type Project struct {
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	Version   int64  `json:"version"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type DeletionOperation struct {
	OperationID       string  `json:"operationId" dynamodbav:"operationId"`
	ProjectID         string  `json:"projectId" dynamodbav:"projectId"`
	State             string  `json:"state" dynamodbav:"state"`
	StartedAt         string  `json:"startedAt" dynamodbav:"startedAt"`
	CompletedAt       *string `json:"completedAt" dynamodbav:"completedAt,omitempty"`
	RetryAfterSeconds int     `json:"retryAfterSeconds" dynamodbav:"retryAfterSeconds"`
}

// record is internal storage, never a response DTO. Omitted fields are removed
// when the project becomes a minimal tombstone.
type record struct {
	PK                     string             `dynamodbav:"PK"`
	SK                     string             `dynamodbav:"SK"`
	Kind                   string             `dynamodbav:"kind"`
	SchemaVersion          int                `dynamodbav:"schemaVersion"`
	UserID                 string             `dynamodbav:"userId,omitempty"`
	Issuer                 string             `dynamodbav:"issuer,omitempty"`
	Subject                string             `dynamodbav:"sub,omitempty"`
	Disabled               bool               `dynamodbav:"disabled,omitempty"`
	DisplayName            string             `dynamodbav:"displayName,omitempty"`
	OwnerID                string             `dynamodbav:"ownerId,omitempty"`
	ProjectID              string             `dynamodbav:"projectId,omitempty"`
	Name                   string             `dynamodbav:"name,omitempty"`
	Version                int64              `dynamodbav:"version"`
	State                  string             `dynamodbav:"state,omitempty"`
	CreatedAt              string             `dynamodbav:"createdAt,omitempty"`
	UpdatedAt              string             `dynamodbav:"updatedAt,omitempty"`
	DeletionEpoch          int64              `dynamodbav:"deletionEpoch,omitempty"`
	InitiatingVersion      int64              `dynamodbav:"initiatingVersion,omitempty"`
	Operation              *DeletionOperation `dynamodbav:"deletionOperation,omitempty"`
	LPK                    string             `dynamodbav:"LPK,omitempty"`
	LSK                    string             `dynamodbav:"LSK,omitempty"`
	DPK                    string             `dynamodbav:"DPK,omitempty"`
	DSK                    string             `dynamodbav:"DSK,omitempty"`
	MaintenanceCursor      map[string]string  `dynamodbav:"maintenanceCursor,omitempty"`
	RecoveryGeneration     string             `dynamodbav:"recoveryGeneration,omitempty"`
	LocalEmptyProjectsOnly bool               `dynamodbav:"localEmptyProjectsOnly,omitempty"`
}

func (r record) user() User { return User{r.UserID, r.DisplayName, r.CreatedAt} }
func (r record) project() Project {
	return Project{r.ProjectID, r.Name, r.Version, r.CreatedAt, r.UpdatedAt}
}
func timestamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000000000Z") }
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("secure randomness unavailable")
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type APIError struct {
	Status    int    `json:"-"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   []any  `json:"details"`
	RequestID string `json:"requestId"`
}

func (e *APIError) Error() string { return e.Code }
func apiError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message, Details: []any{}}
}
func unavailable() *APIError {
	return apiError(http.StatusServiceUnavailable, "service_unavailable", "Service temporarily unavailable.")
}
func missing() *APIError { return apiError(http.StatusNotFound, "not_found", "Resource not found.") }
func forbidden() *APIError {
	return apiError(http.StatusForbidden, "forbidden", "Account unavailable.")
}
func stale() *APIError {
	return apiError(http.StatusPreconditionFailed, "precondition_failed", "Project changed; refresh before retrying.")
}
func conflict() *APIError {
	return apiError(http.StatusConflict, "conflict", "Concurrent state change; refresh before retrying.")
}
