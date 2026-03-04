from langchain_community.document_loaders import TextLoader
from langchain_text_splitters import RecursiveCharacterTextSplitter
from langchain_community.vectorstores import FAISS
from langchain_ollama import OllamaEmbeddings, OllamaLLM
from langchain.chains import RetrievalQA

# 加载文档
loader = TextLoader("/workspace/czk/czk_rag/rag_doc/1.txt")
docs = loader.load()

# 分割文本
text_splitter = RecursiveCharacterTextSplitter(
    chunk_size=1000, chunk_overlap=200)
splits = text_splitter.split_documents(docs)

# 创建向量数据库
embeddings = OllamaEmbeddings(model="deepseek-r1:32b")
vectorstore = FAISS.from_documents(splits, embeddings)

# 构建问答链
qa_chain = RetrievalQA.from_chain_type(
    llm=OllamaLLM(model="qwen:7b"),
    retriever=vectorstore.as_retriever(),
    return_source_documents=True
)

# 提问
query = "帮我给小明这个文档起个文档名字"
result = qa_chain.invoke({"query": query})

print("-" * 50)
print("答案:", result["result"])
print("-" * 50)
print("来源文档:", result["source_documents"][0].page_content[:200])