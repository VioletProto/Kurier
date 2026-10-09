package ownership

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

type keyGenerator interface {
	GenerateDataKey(context.Context, *kms.GenerateDataKeyInput, ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error)
}
type EnvelopeCipher struct {
	kms           keyGenerator
	keyARN, stage string
}

func NewEnvelopeCipher(client *kms.Client, keyARN, stage string) *EnvelopeCipher {
	return &EnvelopeCipher{client, keyARN, stage}
}

type Envelope struct {
	Version    int    `json:"version"`
	Algorithm  string `json:"algorithm"`
	KeyARN     string `json:"keyArn"`
	WrappedKey []byte `json:"wrappedKey"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
	Tag        []byte `json:"tag"`
}

func cryptoContext(stage, project, id string, revision int64) map[string]string {
	return map[string]string{"app": "kurier", "stage": stage, "project": project, "binding": id, "purpose": "saved-secret", "version": jsonNumber(revision)}
}
func jsonNumber(v int64) string { b, _ := json.Marshal(v); return string(b) }
func (c *EnvelopeCipher) encrypt(ctx context.Context, project, id string, revision int64, value []byte) (*Envelope, error) {
	defer clear(value)
	if c == nil || c.kms == nil || len(value) == 0 || len(value) > 8192 {
		return nil, unavailable()
	}
	scope := cryptoContext(c.stage, project, id, revision)
	result, err := c.kms.GenerateDataKey(ctx, &kms.GenerateDataKeyInput{KeyId: aws.String(c.keyARN), KeySpec: kmstypes.DataKeySpecAes256, EncryptionContext: scope})
	if result != nil {
		defer clear(result.Plaintext)
	}
	if err != nil || result == nil || len(result.Plaintext) != 32 || len(result.CiphertextBlob) == 0 || aws.ToString(result.KeyId) != c.keyARN {
		return nil, unavailable()
	}
	block, err := aes.NewCipher(result.Plaintext)
	if err != nil {
		return nil, unavailable()
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, unavailable()
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, unavailable()
	}
	aad, _ := json.Marshal(scope)
	sealed := gcm.Seal(nil, nonce, value, aad)
	e := &Envelope{1, "AES-256-GCM", c.keyARN, result.CiphertextBlob, nonce, sealed[:len(sealed)-16], sealed[len(sealed)-16:]}
	b, _ := json.Marshal(e)
	if len(b) > 16384 {
		return nil, invalidConfiguration()
	}
	return e, nil
}

type SecretMetadata struct {
	SecretID   string `json:"secretId"`
	ProjectID  string `json:"projectId"`
	Revision   int64  `json:"revision"`
	State      string `json:"state"`
	Kind       string `json:"valueKind"`
	HeaderSafe bool   `json:"headerSafe"`
	QuerySafe  bool   `json:"querySafe"`
	Masked     bool   `json:"masked"`
	Mask       string `json:"mask"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}
type protectedRecord struct {
	PK            string    `dynamodbav:"PK"`
	SK            string    `dynamodbav:"SK"`
	Kind          string    `dynamodbav:"kind"`
	SchemaVersion int       `dynamodbav:"schemaVersion"`
	SecretID      string    `dynamodbav:"secretId"`
	ProjectID     string    `dynamodbav:"projectId"`
	Revision      int64     `dynamodbav:"revision"`
	State         string    `dynamodbav:"state"`
	ValueKind     string    `dynamodbav:"valueKind"`
	HeaderSafe    bool      `dynamodbav:"headerSafe"`
	QuerySafe     bool      `dynamodbav:"querySafe"`
	CreatedAt     string    `dynamodbav:"createdAt"`
	UpdatedAt     string    `dynamodbav:"updatedAt"`
	Envelope      *Envelope `dynamodbav:"envelope,omitempty"`
}

func (r protectedRecord) metadata() SecretMetadata {
	return SecretMetadata{r.SecretID, r.ProjectID, r.Revision, r.State, r.ValueKind, r.HeaderSafe, r.QuerySafe, true, "••••••••", r.CreatedAt, r.UpdatedAt}
}
func knownProtected(item map[string]types.AttributeValue, project string) (protectedRecord, bool) {
	var r protectedRecord
	allowed := map[string]bool{}
	for _, k := range []string{"PK", "SK", "kind", "schemaVersion", "secretId", "projectId", "revision", "state", "valueKind", "headerSafe", "querySafe", "createdAt", "updatedAt", "envelope"} {
		allowed[k] = true
	}
	for k := range item {
		if !allowed[k] {
			return r, false
		}
	}
	if attributevalue.UnmarshalMap(item, &r) != nil || r.SchemaVersion != 1 || r.Kind != "savedSecret" || r.ProjectID != project || r.PK != "P#"+project || !bindingUUID.MatchString(r.SecretID) || r.SK != "SECRET#"+r.SecretID || r.Revision < 0 || r.CreatedAt == "" || r.UpdatedAt == "" || (r.ValueKind != "string" && r.ValueKind != "jsonBody" && r.ValueKind != "jsonScalar") {
		return r, false
	}
	if r.State == "revoked" {
		return r, r.Envelope == nil
	}
	if r.State != "active" || r.Envelope == nil {
		return r, false
	}
	e := r.Envelope
	// Re-encode envelope to reject unknown nested attributes before cleanup.
	v, ok := item["envelope"].(*types.AttributeValueMemberM)
	if !ok || len(v.Value) != 7 {
		return r, false
	}
	for _, k := range []string{"Version", "Algorithm", "KeyARN", "WrappedKey", "Nonce", "Ciphertext", "Tag"} {
		if _, ok := v.Value[k]; !ok {
			return r, false
		}
	}
	b, _ := json.Marshal(e)
	return r, e.Version == 1 && e.Algorithm == "AES-256-GCM" && strings.HasPrefix(e.KeyARN, "arn:aws:kms:us-east-2:") && len(e.WrappedKey) > 0 && len(e.WrappedKey) <= 6144 && len(e.Nonce) == 12 && len(e.Tag) == 16 && len(e.Ciphertext) > 0 && len(e.Ciphertext) <= 8192 && len(b) <= 16384
}
func (st *Store) ConfigureProtected(table string, c *EnvelopeCipher) {
	st.protectedTable = table
	st.cipher = c
}
func (st *Store) secretContext(ctx context.Context, user, project string) (record, record, record, error) {
	stage, u, p, err := st.requestContext(ctx, user, project)
	if err != nil {
		return stage, u, p, err
	}
	if (stage.SavedRequestsSchemaVersion != 2 && stage.SavedRequestsSchemaVersion != 3) || stage.ProtectedSecretsSchemaVersion != 1 || st.protectedTable == "" {
		return stage, u, p, unavailable()
	}
	return stage, u, p, nil
}
func (st *Store) secret(ctx context.Context, project, id string) (protectedRecord, error) {
	if !bindingUUID.MatchString(id) || st.protectedTable == "" {
		return protectedRecord{}, missing()
	}
	out, err := st.db.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(st.protectedTable), Key: key("P#"+project, "SECRET#"+id), ConsistentRead: aws.Bool(true)})
	if err != nil {
		return protectedRecord{}, unavailable()
	}
	if len(out.Item) == 0 {
		return protectedRecord{}, missing()
	}
	r, ok := knownProtected(out.Item, project)
	if !ok {
		return r, missing()
	}
	return r, nil
}
func validateSecretValue(kind, value string) (bool, error) {
	if !utf8.ValidString(value) || len(value) == 0 || len(value) > 8192 {
		return false, invalidConfiguration()
	}
	if kind != "string" {
		if uniqueJSON([]byte(value)) != nil {
			return false, invalidConfiguration()
		}
		if kind == "jsonScalar" {
			var v any
			json.Unmarshal([]byte(value), &v)
			switch v.(type) {
			case map[string]any, []any:
				return false, invalidConfiguration()
			}
		}
	}
	return kind == "string" && !control(value, true), nil
}
func (st *Store) secretPut(r protectedRecord, initial bool) (types.TransactWriteItem, error) {
	item, err := attributevalue.MarshalMap(r)
	condition := "attribute_not_exists(PK)"
	var values map[string]types.AttributeValue
	var names map[string]string
	if !initial {
		condition = "#r = :r AND #s = :active AND schemaVersion = :schema AND #k = :kind"
		values = map[string]types.AttributeValue{":r": n(r.Revision - 1), ":active": s("active"), ":schema": n(1), ":kind": s("savedSecret")}
		names = map[string]string{"#r": "revision", "#s": "state", "#k": "kind"}
	}
	return types.TransactWriteItem{Put: &types.Put{TableName: aws.String(st.protectedTable), Item: item, ConditionExpression: aws.String(condition), ExpressionAttributeValues: values, ExpressionAttributeNames: names}}, err
}
func refFailure(err error) error {
	var a *APIError
	if errors.As(err, &a) && a.Status == 404 {
		return apiError(400, "invalid_secret_reference", "Secret reference is not eligible.")
	}
	return err
}
func (st *Store) protectConfiguration(ctx context.Context, project string, c *RequestConfiguration, previous *RequestConfiguration) ([]types.TransactWriteItem, error) {
	var actions []types.TransactWriteItem
	old := map[string]secretSlot{}
	if previous != nil {
		for _, f := range slots(previous) {
			old[f.id] = f
		}
	}
	checked := map[string]bool{}
	remove := map[string]bool{}
	for _, f := range slots(c) {
		w := *f.write
		implicit := w == nil
		if implicit {
			if *f.ref == nil {
				return nil, invalidConfiguration()
			}
			w = &SecretWrite{Action: "preserve", SecretRef: *f.ref}
		}
		if w.Action == "remove" {
			remove[f.id] = true
			continue
		}
		if w.Action == "preserve" {
			prior, ok := old[f.id]
			if !ok || prior.locator != f.locator || *prior.ref == nil || w.SecretRef == nil || (*prior.ref).SecretID != w.SecretRef.SecretID {
				return nil, apiError(400, "invalid_secret_reference", "Preserve must retain an existing binding.")
			}
		}
		if w.Action == "set" {
			headerSafe, err := validateSecretValue(f.kind, *w.Value)
			if err != nil {
				return nil, err
			}
			if (strings.HasPrefix(f.locator, "headers\x00") && !headerSafe) || (strings.HasPrefix(f.locator, "query\x00") && control(*w.Value, false)) {
				return nil, invalidConfiguration()
			}
			id := newID()
			env, err := st.cipher.encrypt(ctx, project, id, 0, []byte(*w.Value))
			if err != nil {
				return nil, err
			}
			now := timestamp(st.now())
			r := protectedRecord{"P#" + project, "SECRET#" + id, "savedSecret", 1, id, project, 0, "active", f.kind, headerSafe, !control(*w.Value, false), now, now, env}
			action, err := st.secretPut(r, true)
			if err != nil {
				return nil, unavailable()
			}
			actions = append(actions, action)
			*f.ref = &SecretReference{id}
		} else {
			if w.SecretRef == nil {
				return nil, invalidConfiguration()
			}
			id := w.SecretRef.SecretID
			r, err := st.secret(ctx, project, id)
			if err != nil {
				var ae *APIError
				if w.Action != "preserve" || !errors.As(err, &ae) || ae.Status != 404 {
					return nil, refFailure(err)
				}
			}
			if err == nil && r.State == "active" {
				if r.ValueKind != f.kind || (strings.HasPrefix(f.locator, "headers\x00") && !r.HeaderSafe) || (strings.HasPrefix(f.locator, "query\x00") && !r.QuerySafe) {
					return nil, apiError(400, "invalid_secret_reference", "Secret reference is not eligible.")
				}
				if !checked[id] {
					checked[id] = true
					actions = append(actions, types.TransactWriteItem{ConditionCheck: &types.ConditionCheck{TableName: aws.String(st.protectedTable), Key: key(r.PK, r.SK), ConditionExpression: aws.String("revision = :r AND #s = :active AND schemaVersion = :schema AND #k = :kind"), ExpressionAttributeNames: map[string]string{"#s": "state", "#k": "kind"}, ExpressionAttributeValues: map[string]types.AttributeValue{":r": n(r.Revision), ":active": s("active"), ":schema": n(1), ":kind": s("savedSecret")}}})
				}
			} else if w.Action != "preserve" {
				return nil, apiError(400, "invalid_secret_reference", "Secret reference is not eligible.")
			}
			*f.ref = w.SecretRef
		}
		*f.write = nil
		*f.masked = true
	}
	if len(remove) > 0 {
		headers := []RequestField{}
		for _, f := range c.Headers {
			if !remove[f.BindingID] {
				headers = append(headers, f)
			}
		}
		c.Headers = headers
		query := []RequestField{}
		for _, f := range c.QueryParameters {
			if !remove[f.BindingID] {
				query = append(query, f)
			}
		}
		c.QueryParameters = query
		if c.Body != nil {
			if remove[c.Body.BindingID] {
				c.Body = nil
			} else {
				fields := []SecretField{}
				for _, f := range c.Body.SecretFields {
					if remove[f.BindingID] {
						continue
					}
					fields = append(fields, f)
				}
				c.Body.SecretFields = fields
			}
		}
	}
	if err := validateConfiguration(c); err != nil {
		return nil, err
	}
	return actions, nil
}
func (st *Store) protectIfEnabled(ctx context.Context, stage record, project string, c *RequestConfiguration, previous *RequestConfiguration) ([]types.TransactWriteItem, error) {
	if len(slots(c)) == 0 {
		return nil, nil
	}
	if (stage.SavedRequestsSchemaVersion != 2 && stage.SavedRequestsSchemaVersion != 3) || stage.ProtectedSecretsSchemaVersion != 1 {
		return nil, unavailable()
	}
	return st.protectConfiguration(ctx, project, c, previous)
}
func (st *Store) ReplaceSecret(ctx context.Context, user, project, id, value string, expected int64) (SecretMetadata, error) {
	stage, u, p, err := st.secretContext(ctx, user, project)
	if err != nil {
		return SecretMetadata{}, err
	}
	r, err := st.secret(ctx, project, id)
	if err != nil || r.State != "active" {
		if err == nil {
			err = missing()
		}
		return SecretMetadata{}, err
	}
	if r.Revision != expected {
		return SecretMetadata{}, requestStale()
	}
	safe, err := validateSecretValue(r.ValueKind, value)
	if err != nil {
		return SecretMetadata{}, err
	}
	if (r.HeaderSafe && !safe) || (r.QuerySafe && control(value, false)) {
		return SecretMetadata{}, invalidConfiguration()
	}
	r.Revision++
	r.UpdatedAt = timestamp(st.now())
	r.Envelope, err = st.cipher.encrypt(ctx, project, id, r.Revision, []byte(value))
	if err != nil {
		return SecretMetadata{}, err
	}
	action, err := st.secretPut(r, false)
	if err != nil {
		return SecretMetadata{}, unavailable()
	}
	if err = st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(u), st.requestGate(p, "active"), action}); err != nil {
		return SecretMetadata{}, st.secretFailure(ctx, user, project, id, expected, err)
	}
	return r.metadata(), nil
}
func (st *Store) RevokeSecret(ctx context.Context, user, project, id string, expected int64) error {
	stage, u, p, err := st.secretContext(ctx, user, project)
	if err != nil {
		return err
	}
	r, err := st.secret(ctx, project, id)
	if err != nil {
		return err
	}
	if r.State != "active" {
		return missing()
	}
	if r.Revision != expected {
		return requestStale()
	}
	r.State = "revoked"
	r.Envelope = nil
	r.Revision++
	r.UpdatedAt = timestamp(st.now())
	action, err := st.secretPut(r, false)
	if err != nil {
		return unavailable()
	}
	if err = st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(u), st.requestGate(p, "active"), action}); err != nil {
		return st.secretFailure(ctx, user, project, id, expected, err)
	}
	return nil
}
func (st *Store) secretFailure(ctx context.Context, user, project, id string, expected int64, original error) error {
	if !isConditional(original) {
		return unavailable()
	}
	if _, _, _, err := st.secretContext(ctx, user, project); err != nil {
		return err
	}
	r, err := st.secret(ctx, project, id)
	if err != nil {
		return err
	}
	if r.Revision != expected {
		return requestStale()
	}
	return conflict()
}
func (st *Store) drainProtected(ctx context.Context, p, stage, work record) (record, bool, error) {
	var start map[string]types.AttributeValue
	if work.ProtectedCursor != "" {
		start = key(p.PK, work.ProtectedCursor)
	}
	page, err := st.db.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(st.protectedTable), KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s(p.PK)}, ConsistentRead: aws.Bool(true), ExclusiveStartKey: start, Limit: aws.Int32(20)})
	if err != nil {
		return p, false, unavailable()
	}
	if len(page.Items) == 0 && start == nil {
		return p, false, nil
	}
	actions := []types.TransactWriteItem{st.stageGuard(stage), st.requestGate(p, "deleting")}
	for _, item := range page.Items {
		r, ok := knownProtected(item, p.ProjectID)
		if !ok {
			continue
		}
		actions = append(actions, types.TransactWriteItem{Delete: &types.Delete{TableName: aws.String(st.protectedTable), Key: key(r.PK, r.SK), ConditionExpression: aws.String("revision = :r AND #s = :s AND schemaVersion = :schema AND #k = :kind"), ExpressionAttributeNames: map[string]string{"#s": "state", "#k": "kind"}, ExpressionAttributeValues: map[string]types.AttributeValue{":r": n(r.Revision), ":s": s(r.State), ":schema": n(1), ":kind": s("savedSecret")}}})
	}
	work.ProtectedCursor = ""
	if v, ok := page.LastEvaluatedKey["SK"].(*types.AttributeValueMemberS); ok {
		work.ProtectedCursor = v.Value
	}
	checkpoint, err := st.put(work, "#v = :v AND #k = :kind", map[string]types.AttributeValue{":v": n(work.Version), ":kind": s("deletionWork")})
	if err != nil {
		return p, false, unavailable()
	}
	checkpoint.Put.ExpressionAttributeNames = map[string]string{"#v": "version", "#k": "kind"}
	checkpoint.Put.Item["version"] = n(work.Version + 1)
	actions = append(actions, checkpoint)
	if err = st.transact(ctx, actions); err != nil {
		if !isConditional(err) {
			return p, false, unavailable()
		}
		return p, false, conflict()
	}
	p.Version++
	return p, len(page.LastEvaluatedKey) > 0, nil
}
