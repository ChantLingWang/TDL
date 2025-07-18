import asyncio

import uvicorn
from fastapi import FastAPI
from requests import Response, Request
from services.mongodb_service import main

app = FastAPI()

@app.get("/")
async def root():
    return {"message": "Hello World"}



if __name__ == "__main__":
    #在api启动时，就自动连接到数据库
    asyncio.run(main())

    uvicorn.run(
        app="api:app",
        host="127.0.0.1",
        port=8000,
        reload=True,
    )
