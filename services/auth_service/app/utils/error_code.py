class ErrorCode:
    def __init__(self,code:int,message:str):
        self.code = code
        self.message = message

    @property
    def code(self):
        return self.code
    
    @property
    def message(self):
        return self.message


class ErrorCodeEnum:
    #用户登录注册相关错误码
    USER_NOT_FOUND = ErrorCode(10001,"用户不存在")
    USER_ALREADY_EXISTS = ErrorCode(10002,"用户已存在")
    USER_PASSWORD_INCORRECT = ErrorCode(10003,"密码错误")
    
    #数据库相关错误码
    DATABASE_CONNECTION_ERROR = ErrorCode(20001,"数据库连接失败")
    
