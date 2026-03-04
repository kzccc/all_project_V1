from langchain_community.document_loaders import TextLoader
from langchain_text_splitters import RecursiveCharacterTextSplitter
from langchain_community.vectorstores import FAISS
from langchain_ollama import OllamaEmbeddings, OllamaLLM
from langchain.chains import RetrievalQA
from langchain.callbacks.base import BaseCallbackHandler
from langchain.schema import LLMResult
import time

# 自定义回调处理器，用于显示处理过程
class MyCustomHandler(BaseCallbackHandler):
    def on_llm_start(self, serialized, prompts, **kwargs):
        """LLM开始运行时调用"""
        print(">>> 开始调用语言模型...")
        
    def on_llm_new_token(self, token, **kwargs):
        """接收到新token时调用"""
        print(token, end="", flush=True)
        
    def on_llm_end(self, response, **kwargs):
        """LLM结束运行时调用"""
        print("\n>>> 模型调用完成")
        
    def on_retriever_start(self, query, **kwargs):
        """检索器开始运行时调用"""
        print(">>> 开始从向量数据库中检索相关信息...")
        
    def on_retriever_end(self, documents, **kwargs):
        """检索器结束运行时调用"""
        print(f">>> 检索完成，找到 {len(documents)} 个相关文档块")

# 加载文档
print("正在加载文档...")
loader = TextLoader("/workspace/czk/chenzekai_rag/rag_doc/1.txt")
docs = loader.load()

# 分割文本
print("正在分割文本...")
text_splitter = RecursiveCharacterTextSplitter(
    chunk_size=1000, chunk_overlap=200)
splits = text_splitter.split_documents(docs)

# 创建向量数据库
print("正在创建向量数据库...")
embeddings = OllamaEmbeddings(model="deepseek-r1:32b")
vectorstore = FAISS.from_documents(splits, embeddings)

# 构建问答链
print("正在构建问答链...")
qa_chain = RetrievalQA.from_chain_type(
    llm=OllamaLLM(model="qwen:7b"),
    retriever=vectorstore.as_retriever(),
    return_source_documents=True,
    chain_type_kwargs={"callbacks": [MyCustomHandler()]}
)

# 提问
print("正在提问...")
query = "介绍王琪的亲子关系"
result = qa_chain.invoke({"query": query})

print("=" * 50)
print("最终答案:", result["result"])
print("=" * 50)
print("来源文档:", result["source_documents"][0].page_content[:200])