import asyncio
from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI
from requests import Response, Request
from services.mongodb_service import db_manager
from api_services.router.oauth import oauth

@asynccontextmanager    #fast API的生命周期上下文管理器，用于在启动和关闭时运行某些函数
async def lifespan(app: FastAPI):
    # 启动时执行
    await db_manager.connect()
    yield       #fast API的生命周期事件，启动时执行某些函数，在field处暂停，直到关闭时执行关闭函数
    # 关闭时执行
    await db_manager.close()

app = FastAPI(lifespan=lifespan)
app.include_router(oauth)

@app.get("/")
async def root():
    return {"message": "Hello World"}

if __name__ == "__main__":
    uvicorn.run(
        app="api:app",
        host="127.0.0.1",
        port=9030,
        reload=False,
    )
