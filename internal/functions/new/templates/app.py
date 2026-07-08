from fastapi import FastAPI

app = FastAPI()


@app.post("/")
async def root_post_handler(payload: dict):
    return {"message": "POST request received", "payload": payload}
