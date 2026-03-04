from langchain_community.llms import Ollama

llm = Ollama(model="qwen:7b", temperature=0.9)

response = llm.invoke("解释量子计算的基本原理")
print("-" * 50)
print("模型回复:")
print(response)
print("-" * 50)

