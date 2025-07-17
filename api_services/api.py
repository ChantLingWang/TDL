import uvicorn
from fastapi import FastAPI
from requests import Response, Request

app = FastAPI()

@app.get("/")
async def root():
    return {"message": "Hello World"}



if __name__ == "__main__":
    uvicorn.run(
        app="api:app",
        host="127.0.0.1",
        port=8000,
        reload=True,
    )
