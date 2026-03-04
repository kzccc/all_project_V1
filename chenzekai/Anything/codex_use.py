import requests

headers = {
    "Authorization": "Bearer YOUR_API_KEY"
}

response = requests.get(
    "https://api.openai.com/v1/usage",
    headers=headers
)
# 返回详细用量信息