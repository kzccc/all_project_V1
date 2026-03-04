from langchain_core.messages import HumanMessage
from langchain_ollama import ChatOllama

chat = ChatOllama(model="qwen:7b", temperature=0.9)

messages = [HumanMessage(content="写一颗线段树")]
response = chat.invoke(messages)
print("-" * 50)
print("AI回复:", response.content)
print("-" * 50)