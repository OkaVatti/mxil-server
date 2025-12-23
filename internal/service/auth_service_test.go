package service

import (
	"context"
	"testing"
	"time"

	"github.com/okavatti/mxil-server/m/internal/auth"
	"github.com/okavatti/mxil-server/m/internal/crypto"
	"github.com/okavatti/mxil-server/m/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

// Mock repositories
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

type MockSessionRepository struct {
	mock.Mock
}

func (m *MockSessionRepository) Create(ctx context.Context, session *models.Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockSessionRepository) GetByToken(ctx context.Context, tokenHash string) (*models.Session, error) {
	args := m.Called(ctx, tokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Session), args.Error(1)
}

func (m *MockSessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Test suite
func TestAuthService_Register(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockSessionRepo := new(MockSessionRepository)

	// Create auth service with mocks
	authService := &AuthService{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
		jwtService:  auth.NewJWTService("test_secret", time.Hour),
	}

	ctx := context.Background()

	// Test case: Successful registration
	t.Run("SuccessfulRegistration", func(t *testing.T) {
		req := RegisterRequest{
			Username:    "testuser",
			Password:    "securepassword123",
			Email:       "test@example.com",
			DisplayName: "Test User",
		}

		// Mock user not found initially
		mockUserRepo.On("GetByUsername", mock.Anything, "testuser").Return((*models.User)(nil), gorm.ErrRecordNotFound)

		// Mock successful user creation
		mockUserRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)

		// Mock successful update with auth method
		mockUserRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)

		resp, err := authService.Register(ctx, req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotEmpty(t, resp.Token)
		assert.Equal(t, "testuser", resp.User.MasterUsername)

		mockUserRepo.AssertExpectations(t)
	})

	// Test case: User already exists
	t.Run("UserAlreadyExists", func(t *testing.T) {
		req := RegisterRequest{
			Username: "existinguser",
			Password: "password123",
			Email:    "existing@example.com",
		}

		existingUser := &models.User{
			ID:             uuid.New(),
			MasterUsername: "existinguser",
		}

		mockUserRepo.On("GetByUsername", mock.Anything, "existinguser").Return(existingUser, nil)

		resp, err := authService.Register(ctx, req)

		assert.Error(t, err)
		assert.Equal(t, ErrUserExists, err)
		assert.Nil(t, resp)

		mockUserRepo.AssertExpectations(t)
	})
}

func TestAuthService_Login(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockSessionRepo := new(MockSessionRepository)

	authService := &AuthService{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
		jwtService:  auth.NewJWTService("test_secret", time.Hour),
	}

	ctx := context.Background()

	t.Run("SuccessfulLogin", func(t *testing.T) {
		req := LoginRequest{
			Username: "testuser",
			Password: "correctpassword",
		}

		// Create mock user with password hash
		userID := uuid.New()
		hashedPassword, _ := crypto.HashPassword("correctpassword")

		user := &models.User{
			ID:             userID,
			MasterUsername: "testuser",
			AuthMethods: []models.AuthMethod{
				{
					ID:             uuid.New(),
					UserID:         userID,
					MethodType:     models.AuthPassword,
					CredentialHash: hashedPassword,
					IsActive:       true,
				},
			},
		}

		mockUserRepo.On("GetByUsername", mock.Anything, "testuser").Return(user, nil)
		mockSessionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Session")).Return(nil)

		resp, err := authService.Login(ctx, req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotEmpty(t, resp.Token)
		assert.Equal(t, userID, resp.User.ID)

		mockUserRepo.AssertExpectations(t)
		mockSessionRepo.AssertExpectations(t)
	})

	t.Run("InvalidCredentials", func(t *testing.T) {
		req := LoginRequest{
			Username: "testuser",
			Password: "wrongpassword",
		}

		userID := uuid.New()
		hashedPassword, _ := crypto.HashPassword("correctpassword")

		user := &models.User{
			ID:             userID,
			MasterUsername: "testuser",
			AuthMethods: []models.AuthMethod{
				{
					ID:             uuid.New(),
					UserID:         userID,
					MethodType:     models.AuthPassword,
					CredentialHash: hashedPassword,
					IsActive:       true,
				},
			},
		}

		mockUserRepo.On("GetByUsername", mock.Anything, "testuser").Return(user, nil)

		resp, err := authService.Login(ctx, req)

		assert.Error(t, err)
		assert.Equal(t, ErrInvalidCredentials, err)
		assert.Nil(t, resp)

		mockUserRepo.AssertExpectations(t)
	})
}
