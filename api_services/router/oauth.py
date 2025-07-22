from fastapi import APIRouter,Request,HTTPException
from pydantic import BaseModel,Field
from services.mongodb_user_service import MongoDBUserService
from services.mongodb_service import MongoDBServiceManager,db_manager


oauth = APIRouter(
    prefix="/oauth",
    tags=["oauth"]
)

class LoginOrRegisterRequest(BaseModel):
    email: str = Field(..., description="用户邮箱")
    password: str = Field(..., description="用户密码")


@oauth.post("/login_or_register",
        summary="用户登录或注册",
        description="用户登录或注册接口",
        response_description="返回登录或注册结果",
    )
async def login_or_register(request:Request,data:LoginOrRegisterRequest):
    try:
        user_service = MongoDBUserService(db_manager)
        is_connected = await db_manager.test_connection()
        if is_connected:
            user = await user_service.login_or_register_user(data.email,data.password)
            #返回用户信息,前端需要用户的各种信息，典型场景，个人资料页面需要大量个人信息，该字段后续一定扩展，但不可返回敏感信息，如密码等
            #在未来，这个返回会包括各种历史记录，和未完成的文档等，这些信息会存储在数据库中，又需要返回给前端进行显示
            return {
                "message":"success",
                "user":{
                    "email":user["email"],
                    #"username":user["username"]
                }
            }
        else:
            raise HTTPException(status_code=500,detail="数据库连接失败")
    except Exception as e:
        raise HTTPException(status_code=500,detail=str(e))
    