需求规格说明书：层次化RAG知识库智能分类系统
一、核心概念：结构化的RAG知识库
当前问题：传统RAG是扁平向量库，检索时缺乏结构意识
解决方案：将层次化分类与RAG检索深度结合

二、系统架构：分类+RAG双引擎
text
                [新文档]
                    ↓
           [六层级自动分类系统]
                    ↓
    [结构化存储]          [向量化处理]
         ↓                    ↓
   文档树管理             向量索引
         ↓                    ↓
    [双路检索] ← [用户提问]
         ↓
    [结果融合] → [上下文增强]
         ↓
      [LLM生成]
         ↓
      [答案返回]
三、详细需求：分类与RAG的深度集成
1. 文档入库流程（分类优先）
text
1. 文档上传 → 内容解析
2. 六层级分类 → 确定存储路径
3. 向量化 → 生成文档向量
4. 双重索引：
   - 向量索引：文档内容向量
   - 结构索引：路径向量 + 层级向量
5. 更新RAG检索器
2. 检索时利用分类结构
传统RAG问题：

"苹果"可能返回水果苹果、公司苹果、手机苹果混杂

缺乏上下文区分

本系统解决方案：

python
# 检索流程优化
1. 用户提问 → 意图识别（判断所属领域/层级）
2. 结构感知检索：
   - 优先在相关层级内检索
   - 跨层级检索时加权处理
3. 结果排序：考虑内容相关度 + 结构匹配度

# 例如用户问："Transformer的注意力机制"
1. 意图识别 → 技术/AI/机器学习/深度学习/Transformer
2. 优先在该路径下检索文档
3. 若无结果，放宽到父节点（深度学习层）
3. 分类标签作为检索元数据
python
# 向量存储时附加元数据
document_metadata = {
    "full_path": "技术/AI/机器学习/深度学习/Transformer/论文",
    "level_1": "技术",
    "level_2": "AI", 
    "level_3": "机器学习",
    "level_4": "深度学习",
    "level_5": "Transformer",
    "level_6": "论文",
    "path_vector": [0.1, 0.2, ...],  # 路径的向量表示
    "content_vector": [0.3, 0.4, ...]  # 内容的向量表示
}

# LangChain检索时可利用元数据过滤
retriever = vectorstore.as_retriever(
    search_kwargs={
        "k": 10,
        "filter": {"level_1": "技术"}  # 可动态指定层级过滤
    }
)
4. 检索策略优化
4.1 层级加权检索

text
得分 = α × 内容相似度 + β × 路径相似度

其中：
- 高层问题（宏观）：β权重高
- 具体问题：α权重高
- 路径相似度 = 提问意图与文档路径的匹配度
4.2 动态检索范围

精确模式：只在指定层级内检索

扩展模式：向父层级扩展

全局模式：全库检索（传统RAG方式）

4.3 路径提示工程

python
# 给LLM的提示中加入路径信息
prompt_template = """
根据以下文档回答问题：

[文档来源]：{document_path}
[文档内容]：{document_content}

问题：{question}

请结合文档的领域背景回答：
"""
5. 分类准确性与RAG质量联动
反馈循环设计：

text
用户提问 → RAG检索 → 用户评价
    ↓
分类不准确 → 检索质量差 → 用户反馈
    ↓
重新训练分类模型 ← 收集错误样本
关键指标：

分类准确率：文档归入正确路径的比例

检索准确率：返回相关文档的比例

问答准确率：生成答案的正确率

结构利用度：检索时利用层级信息的比例

6. LangChain集成点
python
# 自定义Retriever
class HierarchicalRetriever(BaseRetriever):
    def __init__(self, classifier, vector_store):
        self.classifier = classifier  # 分类模型
        self.vector_store = vector_store  # 向量库
        
    def get_relevant_documents(self, query: str) -> List[Document]:
        # 1. 对query进行分类意图识别
        predicted_path = self.classifier.predict_query_category(query)
        
        # 2. 按层级检索
        if predicted_path.confidence > threshold:
            # 优先在预测路径下检索
            results = self.vector_store.search(
                query=query, 
                filter_path=predicted_path
            )
        else:
            # 全库检索
            results = self.vector_store.search(query=query)
            
        return results

# 集成到LangChain Chain
rag_chain = (
    {"context": hierarchical_retriever, "question": RunnablePassthrough()}
    | prompt
    | llm
    | StrOutputParser()
)
7. 部署架构
text
前端界面
    ↓
FastAPI服务层
    ├── 文档上传/分类接口
    ├── 问答接口
    └── 管理接口（人工确认）
    ↓
LangChain处理层
    ├── 分类管道
    ├── 检索管道  
    └── 生成管道
    ↓
存储层
    ├── 向量数据库（Chroma/Qdrant）带层级元数据
    ├── 文档文件存储（按层级目录组织）
    └── 分类模型存储
8. 特别注意事项
8.1 分类粒度与检索平衡

分类太细 → 检索可能漏掉相关文档

分类太粗 → 失去了结构优势

解决方案：检索时智能放宽层级限制

8.2 新文档冷启动问题

新类别文档少，检索时可能被忽略

解决方案：新类别文档加权，确保能被检索到

8.3 跨领域文档处理

一个提问可能涉及多个领域

解决方案：多路径检索 + 结果融合

8.4 版本一致性

分类结构调整后，向量库需要同步更新

解决方案：版本化向量索引 + 增量更新

四、预期收益
检索精度提升：利用结构信息过滤无关文档

答案质量提高：LLM获得更有针对性的上下文

可解释性增强：能说明答案来源于哪个领域/层级

管理效率提升：文档自动归类，便于人工管理

五、一句话总结
"构建一个六层级的智能文档分类系统，与RAG深度集成，实现：文档自动归类存储 + 结构感知检索 + 层级优化问答，每次入库触发重组织，需人工确认，服务于高质量知识库问答。"