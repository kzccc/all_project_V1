from langchain_text_splitters import MarkdownHeaderTextSplitter, RecursiveCharacterTextSplitter


def split_documents(documents, chunk_size: int, chunk_overlap: int):
    splitter = RecursiveCharacterTextSplitter(
        chunk_size=chunk_size,
        chunk_overlap=chunk_overlap,
    )
    markdown_splitter = MarkdownHeaderTextSplitter(
        headers_to_split_on=[
            ("#", "h1"),
            ("##", "h2"),
            ("###", "h3"),
            ("####", "h4"),
        ]
    )

    chunks = []
    for doc in documents:
        source = doc.metadata.get("source", "")
        if source.lower().endswith((".md", ".markdown")):
            header_docs = markdown_splitter.split_text(doc.page_content)
            for header_doc in header_docs:
                header_doc.metadata["source"] = source
            chunks.extend(splitter.split_documents(header_docs))
        else:
            chunks.extend(splitter.split_documents([doc]))

    return chunks
