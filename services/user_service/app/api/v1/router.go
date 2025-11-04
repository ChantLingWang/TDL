package v1

import (
	"github.com/gin-gonic/gin"
)

// Router 用户路由器，类似于Python中的APIRouter
type Router struct {
	Engine *gin.Engine
}

// NewRouter 创建新的用户路由器
func NewRouter() *Router {
	return &Router{}
}

// SetupRoutes 设置路由，类似于Python中的router = APIRouter()
func (r *Router) SetupRoutes() *gin.RouterGroup {
	// 创建API v1路由组，类似于Python中的APIRouter(prefix="/api/v1")
	v1 := r.Engine.Group("/api/v1")
	
	// 用户相关路由，类似于Python中的用户CRUD操作
	users := v1.Group("/users")
	{	
		// 根据用户ID获取用户信息
		users.GET("/:user_id", GetUser)
	}
	
	return v1
}


// GetRouter 获取路由器实例
func GetRouter() *Router {
	return NewRouter()
}
