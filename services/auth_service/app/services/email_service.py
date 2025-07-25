import smtplib
import secrets
import string
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from email.header import Header
from email.utils import formataddr
from app.core.config import settings

class EmailService:
    def __init__(self):
        self.smtp_server = settings.smtp_server
        self.smtp_port = settings.smtp_port
        self.smtp_username = settings.smtp_username
        self.smtp_password = settings.smtp_password
        
    def generate_secure_code(length=6):
    # 大写字母 + 数字
        characters = string.ascii_uppercase + string.digits
        return ''.join(secrets.choice(characters) for _ in range(length))    
        
        
    def send_email(self, to_email: str, subject: str, content: str = None):
        #生成验证码
        code = self.generate_secure_code()
        # 发送邮件
        if subject is None:
            subject = "验证码"
        if content is None:
            content = (
                "您的注册验证码是：{code}\n"
                "请在10分钟内完成验证，如非本人操作，请忽略此邮件。"
            )
        #将验证码格式化后放入content中
        final_content = content.format(code=code)
        #创建邮箱对象
        msg = MIMEMultipart()
        msg['From'] = formataddr(("Chant", self.smtp_username))
        msg['To'] = to_email    
        msg['Subject'] = subject
        msg.attach(MIMEText(final_content, 'plain', 'utf-8'))
        
        #连接邮箱服务器
        server = smtplib.SMTP(self.smtp_server, self.smtp_port)
        server.starttls()
        server.login(self.smtp_username, self.smtp_password)
        server.sendmail(self.smtp_username, to_email, msg.as_string())
        server.quit()
        
        #将code存入redis，并设置TTL
        redis_client = RedisClient()
            