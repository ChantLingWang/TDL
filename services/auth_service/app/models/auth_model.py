from pydantic import BaseModel,Field,EmailStr


class RegisterRequest(BaseModel):
    username: str = Field(..., min_length=3, max_length=10,description="用户名")
    email: EmailStr = Field(..., description="用户邮箱")
    password: str = Field(..., min_length=7, max_length=15,description="用户密码")

class LoginRequest(BaseModel):
    email: EmailStr = Field(..., description="用户邮箱")
    password: str = Field(..., description="用户密码")
    
class SendCodeRequest(BaseModel):
    email: EmailStr = Field(..., description="用户邮箱")