from pydantic import BaseModel,Field

class LoginOrRegisterRequest(BaseModel):
    email: str = Field(..., description="用户邮箱")
    password: str = Field(..., description="用户密码")