package handler

import (
	"net/http"

	"zhixu-backend/internal/dto"
	apperrors "zhixu-backend/internal/errors"
	"zhixu-backend/internal/middleware"
	"zhixu-backend/internal/model"
	"zhixu-backend/internal/response"
	"zhixu-backend/internal/service"
	"zhixu-backend/internal/utils"
	"zhixu-backend/internal/vo"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	AuthService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{
		AuthService: authService,
	}
}

func (h *AuthHandler) Register(r *gin.RouterGroup) {
	r.POST("/register", h.RegisterUser)
	r.POST("/login", h.Login)
	r.GET("/check", middleware.AuthMiddleware(), h.checkLogin)
	r.GET("/profile", middleware.AuthMiddleware(), h.Profile)
}

// RegisterUser godoc
// @Summary 用户注册
// @Description 用户通过用户名、邮箱、密码注册
// @Tags auth
// @Accept json
// @Produce json
// @Param data body dto.RegisterUserReq true "注册参数"
// @Success 200 {object} response.Response{data=nil} "返回统一响应（code=0成功，code!=0失败；data可能为空）"
// @Router /auth/register [post]
func (h *AuthHandler) RegisterUser(c *gin.Context) {
	// 注册用户
	var req dto.RegisterUserReq
	// 绑定 JSON 请求体到 req 结构体
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, "参数错误")
		return
	}
	// 密码哈希
	hashPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, "密码处理失败")
		return
	}
	// 创建新用户
	newUser := &model.UserModel{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hashPassword,
		AvatarURL:    "",
		StorageLimit: 1024 * 1024 * 1024, // 1GB
		StorageUsed:  0,
		Status:       1,
	}
	// 注册用户
	if err := h.AuthService.RegisterUser(newUser); err != nil {
		response.Fail(c, response.CodeServerError)
		return
	}
	// 返回成功响应
	response.Success(c, nil)
}

// Login godoc
// @Summary 用户登录
// @Description 用户通过账号密码登录
// @Tags auth
// @Accept json
// @Produce json
// @Param data body dto.LoginReq true "登录参数"
// @Success 200 {object} response.Response{data=vo.LoginResp} "返回统一响应（code=0成功，code!=0失败；data包含token）"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	// 登录用户
	var req dto.LoginReq
	// 绑定 JSON 请求体到 req 结构体
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, "参数错误")
		return
	}
	// 验证用户
	user, err := h.AuthService.ValidateUser(req.Username, req.Password)
	if err != nil {
		// 根据错误类型返回不同的状态码
		if apperrors.Is(err, apperrors.ErrUserNotFound) || apperrors.Is(err, apperrors.ErrInvalidPassword) {
			response.FailWithStatus(c, http.StatusUnauthorized, response.CodeUnauthorized, "用户名或密码错误")
			return
		}
		response.FailWithStatus(c, http.StatusUnauthorized, response.CodeUnauthorized, err.Error())
		return
	}
	// 生成 JWT 令牌
	token, err := h.AuthService.GenerateToken(user.ID)
	if err != nil {
		response.Fail(c, response.CodeServerError)
		return
	}
	// 返回成功响应
	response.Success(c, vo.LoginResp{
		Token: token,
	})
}

// Profile godoc
// @Summary 获取当前用户信息
// @Description 返回当前登录用户的基本信息与存储用量（通过JWT Token识别）
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=vo.ProfileResp} "返回统一响应（code=0成功；data包含用户信息）"
// @Failure 401 {object} response.Response "未登录（code=1002）"
// @Security ApiKeyAuth
// @Router /auth/profile [get]
func (h *AuthHandler) Profile(c *gin.Context) {
	// 从上下文获取用户ID（AuthMiddleware 已保证存在）
	userID, ok := getCurrentUserID(c)
	if !ok {
		response.FailWithMsg(c, response.CodeUnauthorized, "unauthorized")
		return
	}
	user, err := h.AuthService.GetUserByID(userID)
	if err != nil {
		response.Fail(c, response.CodeServerError)
		return
	}
	response.Success(c, vo.ProfileResp{
		ID:           user.ID,
		Username:     user.Username,
		Email:        user.Email,
		AvatarURL:    user.AvatarURL,
		StorageLimit: user.StorageLimit,
		StorageUsed:  user.StorageUsed,
		CreatedAt:    user.CreatedAt,
	})
}

// checkLogin godoc
// @Summary 检查登录状态
// @Description 验证用户是否已登录（通过JWT Token）
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=nil} "已登录（code=0）"
// @Failure 401 {object} response.Response "未登录（code=1002）"
// @Security ApiKeyAuth
// @Router /auth/check [get]
func (h *AuthHandler) checkLogin(c *gin.Context) {
	// 从上下文获取用户ID
	_, exists := c.Get("user_id")
	if !exists {
		response.FailWithMsg(c, response.CodeUnauthorized, "unauthorized")
		return
	}
	response.Success(c, nil)
}
