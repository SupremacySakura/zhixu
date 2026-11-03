package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"zhixu-backend/internal/dto"
	"zhixu-backend/internal/model"
	"zhixu-backend/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Mock AuthService
type mockAuthService struct {
	validateUserFunc  func(username, password string) (*model.UserModel, error)
	registerUserFunc  func(user *model.UserModel) error
	generateTokenFunc func(userID uint) (string, error)
	getUserByIDFunc   func(userID uint) (*model.UserModel, error)
}

func (m *mockAuthService) ValidateUser(username, password string) (*model.UserModel, error) {
	if m.validateUserFunc != nil {
		return m.validateUserFunc(username, password)
	}
	return nil, errors.New("not implemented")
}

func (m *mockAuthService) RegisterUser(user *model.UserModel) error {
	if m.registerUserFunc != nil {
		return m.registerUserFunc(user)
	}
	return nil
}

func (m *mockAuthService) GenerateToken(userID uint) (string, error) {
	if m.generateTokenFunc != nil {
		return m.generateTokenFunc(userID)
	}
	return "", nil
}

func (m *mockAuthService) GetUserByID(userID uint) (*model.UserModel, error) {
	if m.getUserByIDFunc != nil {
		return m.getUserByIDFunc(userID)
	}
	return nil, nil
}

func setupAuthHandlerTest() (*gin.Engine, *mockAuthService, *AuthHandler) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	mockSvc := &mockAuthService{}
	handler := NewAuthHandler(mockSvc)
	return router, mockSvc, handler
}

// Test Login Handler - 参数验证
func TestLogin_InvalidJSON(t *testing.T) {
	router, _, handler := setupAuthHandlerTest()
	router.POST("/login", handler.Login)

	// 发送无效的 JSON
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "1001") // CodeInvalidParam
}

func TestLogin_EmptyUsername(t *testing.T) {
	router, _, handler := setupAuthHandlerTest()
	router.POST("/login", handler.Login)

	// 空用户名
	reqBody, _ := json.Marshal(dto.LoginReq{
		Username: "",
		Password: "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogin_EmptyPassword(t *testing.T) {
	router, _, handler := setupAuthHandlerTest()
	router.POST("/login", handler.Login)

	// 空密码
	reqBody, _ := json.Marshal(dto.LoginReq{
		Username: "testuser",
		Password: "",
	})
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogin_UserNotFound(t *testing.T) {
	router, mockSvc, handler := setupAuthHandlerTest()
	mockSvc.validateUserFunc = func(username, password string) (*model.UserModel, error) {
		return nil, errors.New("用户不存在")
	}
	router.POST("/login", handler.Login)

	reqBody, _ := json.Marshal(dto.LoginReq{
		Username: "nonexistent",
		Password: "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "1002")
}

func TestLogin_WrongPassword(t *testing.T) {
	router, mockSvc, handler := setupAuthHandlerTest()
	hashedPassword, _ := utils.HashPassword("correctpassword")
	mockSvc.validateUserFunc = func(username, password string) (*model.UserModel, error) {
		if password == "correctpassword" {
			return &model.UserModel{ID: 1, Username: username, PasswordHash: hashedPassword}, nil
		}
		return nil, errors.New("密码错误")
	}
	router.POST("/login", handler.Login)

	reqBody, _ := json.Marshal(dto.LoginReq{
		Username: "testuser",
		Password: "wrongpassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "1002")
}

func TestLogin_Success(t *testing.T) {
	router, mockSvc, handler := setupAuthHandlerTest()
	hashedPassword, _ := utils.HashPassword("password123")
	mockSvc.validateUserFunc = func(username, password string) (*model.UserModel, error) {
		return &model.UserModel{ID: 1, Username: username, PasswordHash: hashedPassword}, nil
	}
	mockSvc.generateTokenFunc = func(userID uint) (string, error) {
		return "test_token_12345", nil
	}
	router.POST("/login", handler.Login)

	reqBody, _ := json.Marshal(dto.LoginReq{
		Username: "testuser",
		Password: "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "0")     // CodeSuccess
	assert.Contains(t, w.Body.String(), "token") // 返回token
}

// Test Register Handler - 参数验证
func TestRegister_InvalidJSON(t *testing.T) {
	router, _, handler := setupAuthHandlerTest()
	router.POST("/register", handler.RegisterUser)

	// 发送无效的 JSON
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "1001")
}

func TestRegister_EmptyUsername(t *testing.T) {
	router, mockSvc, handler := setupAuthHandlerTest()
	mockSvc.registerUserFunc = func(user *model.UserModel) error {
		return nil
	}
	router.POST("/register", handler.RegisterUser)

	// 空用户名
	reqBody, _ := json.Marshal(dto.RegisterUserReq{
		Username: "",
		Email:    "test@example.com",
		Password: "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRegister_EmptyEmail(t *testing.T) {
	router, mockSvc, handler := setupAuthHandlerTest()
	mockSvc.registerUserFunc = func(user *model.UserModel) error {
		return nil
	}
	router.POST("/register", handler.RegisterUser)

	// 空邮箱
	reqBody, _ := json.Marshal(dto.RegisterUserReq{
		Username: "testuser",
		Email:    "",
		Password: "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRegister_EmptyPassword(t *testing.T) {
	router, mockSvc, handler := setupAuthHandlerTest()
	mockSvc.registerUserFunc = func(user *model.UserModel) error {
		return nil
	}
	router.POST("/register", handler.RegisterUser)

	// 空密码
	reqBody, _ := json.Marshal(dto.RegisterUserReq{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "",
	})
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRegister_Success(t *testing.T) {
	router, mockSvc, handler := setupAuthHandlerTest()
	mockSvc.registerUserFunc = func(user *model.UserModel) error {
		return nil
	}
	router.POST("/register", handler.RegisterUser)

	reqBody, _ := json.Marshal(dto.RegisterUserReq{
		Username: "newuser",
		Email:    "newuser@example.com",
		Password: "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "0") // CodeSuccess
}

func TestRegister_ServiceError(t *testing.T) {
	router, mockSvc, handler := setupAuthHandlerTest()
	mockSvc.registerUserFunc = func(user *model.UserModel) error {
		return errors.New("用户名已存在")
	}
	router.POST("/register", handler.RegisterUser)

	reqBody, _ := json.Marshal(dto.RegisterUserReq{
		Username: "existinguser",
		Email:    "user@example.com",
		Password: "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "1004") // CodeServerError
}

// Test checkLogin Handler
func TestCheckLogin_NoUserID(t *testing.T) {
	router, _, handler := setupAuthHandlerTest()
	router.GET("/check", handler.checkLogin)

	req := httptest.NewRequest(http.MethodGet, "/check", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "1002") // CodeUnauthorized
}

func TestCheckLogin_WithUserID(t *testing.T) {
	router, _, handler := setupAuthHandlerTest()
	router.GET("/check", func(c *gin.Context) {
		c.Set("user_id", uint(123))
		handler.checkLogin(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/check", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "0") // CodeSuccess
}

// Test Profile Handler
func TestProfile_NoUserID(t *testing.T) {
	router, _, handler := setupAuthHandlerTest()
	router.GET("/profile", handler.Profile)

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "1002") // CodeUnauthorized
}

func TestProfile_Success(t *testing.T) {
	router, mockSvc, handler := setupAuthHandlerTest()
	mockSvc.getUserByIDFunc = func(userID uint) (*model.UserModel, error) {
		return &model.UserModel{
			ID:           userID,
			Username:     "testuser",
			Email:        "test@example.com",
			AvatarURL:    "",
			StorageLimit: 1024,
			StorageUsed:  512,
		}, nil
	}
	router.GET("/profile", func(c *gin.Context) {
		c.Set("user_id", uint(1))
		handler.Profile(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"code":0`)
	assert.Contains(t, w.Body.String(), `"username":"testuser"`)
	assert.Contains(t, w.Body.String(), `"email":"test@example.com"`)
	assert.Contains(t, w.Body.String(), `"storage_limit":1024`)
	assert.Contains(t, w.Body.String(), `"storage_used":512`)
	assert.NotContains(t, w.Body.String(), "password")
}

func TestProfile_ServiceError(t *testing.T) {
	router, mockSvc, handler := setupAuthHandlerTest()
	mockSvc.getUserByIDFunc = func(userID uint) (*model.UserModel, error) {
		return nil, errors.New("user not found")
	}
	router.GET("/profile", func(c *gin.Context) {
		c.Set("user_id", uint(999))
		handler.Profile(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "1004") // CodeServerError
}

// Test Login with Token Generation Error
func TestLogin_TokenGenerationError(t *testing.T) {
	router, mockSvc, handler := setupAuthHandlerTest()
	hashedPassword, _ := utils.HashPassword("password123")
	mockSvc.validateUserFunc = func(username, password string) (*model.UserModel, error) {
		return &model.UserModel{ID: 1, Username: username, PasswordHash: hashedPassword}, nil
	}
	mockSvc.generateTokenFunc = func(userID uint) (string, error) {
		return "", errors.New("token generation failed")
	}
	router.POST("/login", handler.Login)

	reqBody, _ := json.Marshal(dto.LoginReq{
		Username: "testuser",
		Password: "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "1004") // CodeServerError
}
