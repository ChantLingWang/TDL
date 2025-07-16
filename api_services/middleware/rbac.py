from requests import Request
from fastapi import APIRouter
import uvicorn

oauth = APIRouter()

@oauth.get("/oauth")
async def oauth(request: Request, data=None):
    return {"data": data}