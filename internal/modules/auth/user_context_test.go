package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/config"
	infraJWT "github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/jwt"
	infraMail "github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/mail"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type contextRepositoryStub struct {
	AuthRepository
	user          *User
	organizations []InitiableOrganizationResponse
	listError     error
	listCalls     int
}

func (r *contextRepositoryStub) FindUserByID(context.Context, uuid.UUID) (*User, error) {
	return r.user, nil
}
func (r *contextRepositoryStub) FindUserByEmail(context.Context, string) (*User, error) {
	return r.user, nil
}
func (r *contextRepositoryStub) ListInitiableOrganizations(context.Context, uuid.UUID) ([]InitiableOrganizationResponse, error) {
	r.listCalls++
	return r.organizations, r.listError
}
func (r *contextRepositoryStub) CreateToken(context.Context, *UserToken) error { return nil }
func (r *contextRepositoryStub) FindToken(context.Context, string, TokenType) (*UserToken, error) {
	return &UserToken{UserID: r.user.ID, Token: "refresh", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

type roleJWTSpy struct{ roles []string }

func (j *roleJWTSpy) GenerateAccessToken(_ uuid.UUID, _, _, role string) (string, error) {
	j.roles = append(j.roles, role)
	return "token", nil
}
func (j *roleJWTSpy) ValidateToken(string) (*infraJWT.Claims, error) {
	return nil, errors.New("unused")
}

func contextUser(role SystemRole) *User {
	id := uuid.New()
	return &User{ID: id, IsActive: true, SystemRole: role, Profile: &Profile{ID: id, Username: "test-user", FullName: "Test user"}}
}

func TestCurrentUserOrganizationContext(t *testing.T) {
	for _, role := range []SystemRole{SystemRoleUser, SystemRoleAdmin} {
		t.Run(string(role), func(t *testing.T) {
			org := InitiableOrganizationResponse{ID: uuid.New(), Name: "Komunitas A", Slug: "komunitas-a"}
			repo := &contextRepositoryStub{user: contextUser(role), organizations: []InitiableOrganizationResponse{org}}
			service := NewAuthService(repo, nil, nil, &config.Config{})
			got, err := service.GetCurrentUser(context.Background(), repo.user.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != repo.user.ID || got.Username != "test-user" || got.SystemRole != role {
				t.Fatalf("identity mismatch: %+v", got)
			}
			if role == SystemRoleUser && (len(got.InitiableOrganizations) != 0 || repo.listCalls != 0) {
				t.Fatal("ordinary user received organization privileges")
			}
			if role == SystemRoleAdmin && (len(got.InitiableOrganizations) != 1 || got.InitiableOrganizations[0].ID != org.ID) {
				t.Fatal("admin organization context missing")
			}
		})
	}
}

func TestCurrentUserEmptyAndFailedOrganizationContext(t *testing.T) {
	repo := &contextRepositoryStub{user: contextUser(SystemRoleAdmin)}
	service := NewAuthService(repo, nil, nil, &config.Config{})
	got, err := service.GetCurrentUser(context.Background(), repo.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if string(payload["initiable_organizations"]) != "[]" {
		t.Fatalf("empty list must be [], got %s", raw)
	}
	if _, exists := payload["capabilities"]; exists {
		t.Fatal("management capabilities leaked to consumer context")
	}
	repo.listError = errors.New("database unavailable")
	if got, err := service.GetCurrentUser(context.Background(), repo.user.ID); err == nil || got != nil {
		t.Fatal("failed lookup must not grant or reuse organization rights")
	}
	repo.listError = nil
	repo.user.IsActive = false
	if got, err := service.GetCurrentUser(context.Background(), repo.user.ID); err == nil || got != nil {
		t.Fatal("inactive user must not get account context")
	}
}

func TestTokenIssuanceUsesCurrentPersistedRole(t *testing.T) {
	user := contextUser(SystemRoleAdmin)
	hash, err := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user.PasswordHash = string(hash)
	repo := &contextRepositoryStub{user: user}
	spy := &roleJWTSpy{}
	service := NewAuthService(repo, spy, nil, &config.Config{})
	if _, err := service.Login(context.Background(), LoginRequest{Email: "test@example.com", Password: "Password123!"}); err != nil {
		t.Fatal(err)
	}
	user.SystemRole = SystemRoleUser
	if _, err := service.RefreshToken(context.Background(), RefreshTokenRequest{RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}
	if len(spy.roles) != 2 || spy.roles[0] != "admin" || spy.roles[1] != "user" {
		t.Fatalf("stale or hardcoded roles: %v", spy.roles)
	}
}

type registrationContextRepository struct {
	contextRepositoryStub
	created *User
}

func (*registrationContextRepository) FindUserByEmail(context.Context, string) (*User, error) {
	return nil, gorm.ErrRecordNotFound
}
func (*registrationContextRepository) FindUserByUsername(context.Context, string) (*User, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *registrationContextRepository) CreateUser(_ context.Context, user *User) error {
	r.created = user
	return nil
}

type contextMailer struct{ infraMail.Mailer }

func (contextMailer) SendVerificationEmail(string, string, string) error { return nil }

func TestPublicRegistrationCannotGrantOrganizationPrivileges(t *testing.T) {
	var request RegisterRequest
	if err := json.Unmarshal([]byte(`{"name":"Test user","email":"test@example.com","password":"Password123!","confirmPassword":"Password123!","system_role":"admin","initiable_organizations":[{"id":"bc9e7805-6000-4e0e-9d00-95fc8b85fe6a"}]}`), &request); err != nil {
		t.Fatal(err)
	}
	repo := &registrationContextRepository{}
	service := NewAuthService(repo, nil, contextMailer{}, &config.Config{})
	response, err := service.Register(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if repo.created.SystemRole != SystemRoleUser || response.SystemRole != SystemRoleUser || repo.listCalls != 0 {
		t.Fatal("public registration granted privileges")
	}
}

func TestCurrentUserHTTPContractAndSafeErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &contextRepositoryStub{user: contextUser(SystemRoleAdmin), organizations: []InitiableOrganizationResponse{{ID: uuid.New(), Name: "Komunitas A", Slug: "komunitas-a"}}}
	handler := NewAuthHandler(NewAuthService(repo, nil, nil, &config.Config{}))
	router := gin.New()
	router.GET("/me", func(c *gin.Context) { c.Set("user_id", repo.user.ID.String()); handler.Me(c) })
	request := func() *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/me", nil))
		return recorder
	}
	response := request()
	if response.Code != 200 {
		t.Fatalf("unexpected status %d", response.Code)
	}
	var envelope struct {
		Status int                 `json:"status"`
		Data   CurrentUserResponse `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Status != 200 || envelope.Data.SystemRole != SystemRoleAdmin || len(envelope.Data.InitiableOrganizations) != 1 {
		t.Fatal("HTTP context contract mismatch")
	}
	repo.listError = errors.New("SQLSTATE secret database")
	response = request()
	if response.Code != 500 || strings.Contains(response.Body.String(), "SQLSTATE") || strings.Contains(response.Body.String(), "secret") {
		t.Fatal("context error leaks internals")
	}
	repo.listError = nil
	repo.user.IsActive = false
	if response = request(); response.Code != 403 {
		t.Fatalf("inactive account status %d", response.Code)
	}
}
