package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/VioletProto/Kurier/services/api/internal/browsersession"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	s, e := configure(ctx)
	cancel()
	if e != nil {
		log.Fatal("browser session configuration unavailable")
	}
	if s.Local {
		server := &http.Server{Addr: "127.0.0.1:" + fallback("PORT", "8081"), Handler: s, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 12 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 32 * 1024}
		log.Print("local browser session broker ready")
		if e = server.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			log.Fatal("browser session broker stopped")
		}
		return
	}
	lambda.Start(func(ctx context.Context, raw json.RawMessage) (any, error) {
		var dispatch struct {
			Source      string `json:"source"`
			Maintenance bool   `json:"maintenance"`
		}
		if json.Unmarshal(raw, &dispatch) != nil {
			return failure(), nil
		}
		if dispatch.Source == "aws.events" || dispatch.Maintenance {
			if e := s.Maintain(ctx); e != nil {
				return nil, errors.New("browser session cleanup pending")
			}
			return map[string]bool{"complete": true}, nil
		}
		var event events.APIGatewayV2HTTPRequest
		if json.Unmarshal(raw, &event) != nil {
			return failure(), nil
		}
		return proxy(s, ctx, event), nil
	})
}
func fallback(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
func configure(ctx context.Context) (*browsersession.Service, error) {
	local := os.Getenv("KURIER_SESSION_LOCAL") == "true"
	stage := os.Getenv("KURIER_STAGE")
	origin := fallback("KURIER_SESSION_FRONTEND_ORIGIN", os.Getenv("KURIER_HOSTED_FRONTEND_ORIGIN"))
	u, e := url.Parse(origin)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.String() != u.Scheme+"://"+u.Host {
		return nil, errors.New("invalid session origin")
	}
	options := []func(*config.LoadOptions) error{config.WithRegion("us-east-2"), config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} })}
	if local {
		if stage != "local" || !(u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1") {
			return nil, errors.New("invalid local configuration")
		}
		options = append(options, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")))
	} else {
		if stage != "dev-api" || os.Getenv("AWS_REGION") != "us-east-2" || os.Getenv("KURIER_DYNAMODB_ENDPOINT") != "" || os.Getenv("KURIER_SESSION_ENCRYPTION_KEY_BASE64") != "" {
			return nil, errors.New("invalid cloud configuration")
		}
		if e = os.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "true"); e != nil {
			return nil, e
		}
	}
	cfg, e := config.LoadDefaultConfig(ctx, options...)
	if e != nil {
		return nil, e
	}
	db := dynamodb.NewFromConfig(cfg)
	table := os.Getenv("KURIER_SESSION_TABLE")
	control := os.Getenv("KURIER_CONTROL_TABLE")
	if table == "" || control == "" {
		return nil, errors.New("session tables required")
	}
	var cipher browsersession.Cipher
	secret := ""
	if local {
		endpoint, e := url.Parse(os.Getenv("KURIER_DYNAMODB_ENDPOINT"))
		if e != nil || endpoint.Scheme != "http" || !(endpoint.Hostname() == "localhost" || endpoint.Hostname() == "127.0.0.1" || endpoint.Hostname() == "::1") || endpoint.Port() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.Path != "" {
			return nil, errors.New("loopback DynamoDB required")
		}
		db = dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) { o.BaseEndpoint = aws.String(endpoint.String()) })
		key, e := base64.StdEncoding.Strict().DecodeString(os.Getenv("KURIER_SESSION_ENCRYPTION_KEY_BASE64"))
		if e != nil || len(key) != 32 {
			return nil, errors.New("local encryption key required")
		}
		cipher = browsersession.LocalCipher{Key: key}
		_, e = db.CreateTable(ctx, &dynamodb.CreateTableInput{TableName: aws.String(table), BillingMode: dt.BillingModePayPerRequest, KeySchema: []dt.KeySchemaElement{{AttributeName: aws.String("PK"), KeyType: dt.KeyTypeHash}}, AttributeDefinitions: []dt.AttributeDefinition{{AttributeName: aws.String("PK"), AttributeType: dt.ScalarAttributeTypeS}}})
		if e != nil {
			var exists *dt.ResourceInUseException
			if !errors.As(e, &exists) {
				return nil, errors.New("local session table unavailable")
			}
		}
	} else {
		keyARN := os.Getenv("KURIER_SESSION_KEY_ARN")
		if !strings.HasPrefix(keyARN, "arn:aws:kms:us-east-2:") {
			return nil, errors.New("session key required")
		}
		cipher = &browsersession.EnvelopeCipher{Client: kms.NewFromConfig(cfg), KeyARN: keyARN, Stage: stage}
		parameter := os.Getenv("KURIER_SESSION_ORIGIN_SECRET_PARAMETER")
		if parameter == "" {
			return nil, errors.New("origin proof parameter required")
		}
		p, e := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String(parameter), WithDecryption: aws.Bool(true)})
		if e != nil || p.Parameter == nil || aws.ToString(p.Parameter.Value) == "" {
			return nil, errors.New("origin proof unavailable")
		}
		mac := hmac.New(sha256.New, []byte(aws.ToString(p.Parameter.Value)))
		mac.Write([]byte("kurier:dev-api:browser-session-origin:v1"))
		secret = base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	}
	verifier, e := browsersession.NewVerifier(os.Getenv("KURIER_COGNITO_ISSUER"), os.Getenv("KURIER_COGNITO_CLIENT_ID"))
	if e != nil {
		return nil, e
	}
	return &browsersession.Service{Store: &browsersession.DynamoStore{Client: db, Table: table, Control: control, Stage: stage}, Cipher: cipher, Provider: browsersession.NewProvider(os.Getenv("KURIER_COGNITO_CLIENT_ID")), Verifier: verifier, Origin: origin, OriginSecret: secret, Local: local}, nil
}
func failure() events.APIGatewayV2HTTPResponse {
	return events.APIGatewayV2HTTPResponse{StatusCode: 400, Headers: map[string]string{"content-type": "application/json", "cache-control": "no-store"}, Body: `{"error":{"code":"invalid_request","message":"Invalid session request.","details":[]}}`}
}
func proxy(handler http.Handler, ctx context.Context, event events.APIGatewayV2HTTPRequest) events.APIGatewayV2HTTPResponse {
	if event.Version != "2.0" || !strings.HasPrefix(event.RawPath, "/") || event.RawQueryString != "" || len(event.Body) > 24*1024 {
		return failure()
	}
	body := event.Body
	if event.IsBase64Encoded {
		raw, e := base64.StdEncoding.Strict().DecodeString(body)
		if e != nil {
			return failure()
		}
		body = string(raw)
	}
	if len(body) > 16*1024 {
		return failure()
	}
	u := &url.URL{Scheme: "https", Host: "broker.invalid", Path: event.RawPath}
	r, e := http.NewRequestWithContext(ctx, event.RequestContext.HTTP.Method, u.String(), strings.NewReader(body))
	if e != nil {
		return failure()
	}
	r.URL.RawPath = event.RawPath
	for k, v := range event.Headers {
		r.Header.Set(k, v)
	}
	if len(event.Cookies) > 0 {
		r.Header.Set("Cookie", strings.Join(event.Cookies, "; "))
	}
	r.RemoteAddr = event.RequestContext.HTTP.SourceIP
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	result := w.Result()
	defer result.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(result.Body, 32769))
	if e != nil || len(raw) > 32768 {
		return failure()
	}
	headers := map[string]string{}
	for k, v := range result.Header {
		if !strings.EqualFold(k, "Set-Cookie") {
			headers[k] = strings.Join(v, ", ")
		}
	}
	return events.APIGatewayV2HTTPResponse{StatusCode: result.StatusCode, Headers: headers, Body: string(raw), Cookies: result.Header.Values("Set-Cookie")}
}
