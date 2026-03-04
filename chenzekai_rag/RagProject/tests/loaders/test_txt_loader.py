from pathlib import Path

from loaders.txt_loader import TxtLoader


def test_txt_loader_reads_text() -> None:
    sample_path = Path(__file__).resolve().parents[1] / "fixtures" / "sample.txt"
    loader = TxtLoader()
    document = loader.load(str(sample_path))

    assert document.doc_id == "sample"
    assert "sample document" in document.text
    assert document.metadata["source_type"] == "txt"
