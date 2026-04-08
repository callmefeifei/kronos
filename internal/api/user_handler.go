package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pstrr/kronos/internal/model"
	"github.com/pstrr/kronos/internal/store"
)

// UserHandler handles user management endpoints (admin only).
type UserHandler struct {
	userStore *store.UserStore
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(userStore *store.UserStore) *UserHandler {
	return &UserHandler{userStore: userStore}
}

type createUserRequest struct {
	Username    string `json:"username" binding:"required,min=2,max=64"`
	Password    string `json:"password" binding:"required,min=6"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role" binding:"omitempty,oneof=admin user"`
}

type updateUserRequest struct {
	DisplayName *string `json:"display_name"`
	Password    *string `json:"password" binding:"omitempty,min=6"`
	Role        *string `json:"role" binding:"omitempty,oneof=admin user"`
	Status      *string `json:"status" binding:"omitempty,oneof=active disabled"`
}

// List returns a paginated list of users.
func (h *UserHandler) List(c *gin.Context) {
	page, size := parsePagination(c)

	users, total, err := h.userStore.List(page, size)
	if err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to list users")
		return
	}

	PagedSuccess(c, users, total, page, size)
}

// Create creates a new user.
func (h *UserHandler) Create(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
		return
	}

	// Check for duplicate username.
	if existing, _ := h.userStore.GetByUsername(req.Username); existing != nil {
		Error(c, http.StatusConflict, 409, "username already exists")
		return
	}

	user := &model.User{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Role:        req.Role,
		Status:      "active",
	}
	if user.Role == "" {
		user.Role = "user"
	}

	if err := user.SetPassword(req.Password); err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to hash password")
		return
	}

	if err := h.userStore.Create(user); err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to create user")
		return
	}

	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "ok",
		Data:    user,
	})
}

// Update modifies an existing user.
func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid user id")
		return
	}

	user, err := h.userStore.GetByID(id)
	if err != nil {
		Error(c, http.StatusNotFound, 404, "user not found")
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
		return
	}

	if req.DisplayName != nil {
		user.DisplayName = *req.DisplayName
	}
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.Status != nil {
		user.Status = *req.Status
	}
	if req.Password != nil {
		if err := user.SetPassword(*req.Password); err != nil {
			Error(c, http.StatusInternalServerError, 500, "failed to hash password")
			return
		}
	}

	if err := h.userStore.Update(user); err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to update user")
		return
	}

	Success(c, user)
}

// Delete removes a user.
func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid user id")
		return
	}

	if _, err := h.userStore.GetByID(id); err != nil {
		Error(c, http.StatusNotFound, 404, "user not found")
		return
	}

	if err := h.userStore.Delete(id); err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to delete user")
		return
	}

	Success(c, nil)
}

// parsePagination extracts page and size query params with defaults.
func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
