"""
뉴스 제목 감성 분류기 — 한국어·영어 금융 감성 모델 두 개를 CPU로 돌린다.

왜 별 컨테이너인가
    번역기(translator)와 같은 이유다. 모델 적재에 수십 초가 걸리고 추론이 CPU를
    쓰므로 API 프로세스 안에 두면 분류 한 묶음이 시세 응답을 밀어낸다. 여기 두면
    이상할 때 `docker compose restart sentiment` 하나로 끝나고, 그동안 Go 쪽 배치는
    라벨을 못 붙인 채 다음 회차를 기다린다 — 심리 축은 「없음」으로 남을 뿐 틀리지 않는다.

왜 GPU를 쓰지 않나
    운영 이전 대상(OCI Always Free)은 GPU 없는 ARM 2코어다. BERT-base 두 개는 CPU로
    하루 5,500건을 몇 분 안에 끝낸다.

모델 (2026-09-23 사용자 결정)
    ko  snunlp/KR-FinBert-SC  — 금융 뉴스 제목 44만 건으로 학습, 3분류
    en  ProsusAI/finbert      — Financial PhraseBank, 3분류
    두 모델 모두 가중치가 pickle(`pytorch_model.bin`)뿐이다. torch>=2.6은
    `torch.load`의 기본값이 `weights_only=True`라 임의 코드를 실행하지 않는다 —
    requirements.txt가 그 하한을 고정한다. 아래 `_assert_weights_only`가 기동 때
    한 번 더 확인한다.

API
    GET  /health          → {"ok": true, "models": {...}}
    POST /classify        {"items": [{"id": "...", "text": "...", "lang": "ko|en"}]}
                          → {"results": [{"id", "label", "score", "model"}]}
    score = P(positive) − P(negative), −1..1. 중립만 있는 제목은 0 근처다.
"""

import inspect
import json
import os
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import torch
from transformers import AutoModelForSequenceClassification, AutoTokenizer

MODELS = {
    "ko": os.environ.get("SENTIMENT_MODEL_KO", "snunlp/KR-FinBert-SC"),
    "en": os.environ.get("SENTIMENT_MODEL_EN", "ProsusAI/finbert"),
}
MAX_ITEMS = 256  # 한 요청의 상한 — Go 배치는 64건씩 보낸다.
MAX_CHARS = 512  # 제목은 이보다 짧다. 본문이 섞여 들어와도 여기서 자른다.

torch.set_num_threads(int(os.environ.get("SENTIMENT_THREADS", "2")))


def _assert_weights_only():
    """torch.load가 기본으로 weights_only=True인지 확인한다(pickle 코드 실행 차단)."""
    default = inspect.signature(torch.load).parameters.get("weights_only")
    if default is None or default.default not in (True, None):
        raise SystemExit(f"torch {torch.__version__}: weights_only default is not True — refuse to load pickle weights")
    major, minor = (int(x) for x in torch.__version__.split(".")[:2])
    if (major, minor) < (2, 6):
        raise SystemExit(f"torch {torch.__version__} < 2.6 — weights_only is not the default")


class Classifier:
    def __init__(self, name: str):
        self.name = name
        self.tokenizer = AutoTokenizer.from_pretrained(name)
        self.model = AutoModelForSequenceClassification.from_pretrained(name, use_safetensors=False)
        self.model.eval()
        labels = {int(k): v.lower() for k, v in self.model.config.id2label.items()}
        self.pos = next(i for i, v in labels.items() if v.startswith("pos"))
        self.neg = next(i for i, v in labels.items() if v.startswith("neg"))
        self.labels = labels
        self.lock = threading.Lock()  # torch 추론은 스레드 하나씩 — 코어 수가 적다.

    def classify(self, texts):
        with self.lock, torch.inference_mode():
            enc = self.tokenizer(texts, padding=True, truncation=True, max_length=96, return_tensors="pt")
            probs = torch.softmax(self.model(**enc).logits, dim=-1)
        out = []
        for row in probs.tolist():
            best = max(range(len(row)), key=lambda i: row[i])
            out.append({"label": self.labels[best], "score": round(row[self.pos] - row[self.neg], 4)})
        return out


_assert_weights_only()
CLASSIFIERS = {lang: Classifier(name) for lang, name in MODELS.items()}
print(f"sentiment: loaded {MODELS} on torch {torch.__version__}", flush=True)


class Handler(BaseHTTPRequestHandler):
    def _json(self, code, body):
        data = json.dumps(body, ensure_ascii=False).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path == "/health":
            return self._json(200, {"ok": True, "models": MODELS})
        return self._json(404, {"error": "not found"})

    def do_POST(self):
        if self.path != "/classify":
            return self._json(404, {"error": "not found"})
        try:
            length = int(self.headers.get("Content-Length", "0"))
            items = json.loads(self.rfile.read(length)).get("items", [])
        except (ValueError, AttributeError):
            return self._json(400, {"error": "bad json"})
        if not isinstance(items, list) or len(items) > MAX_ITEMS:
            return self._json(400, {"error": f"items must be a list of at most {MAX_ITEMS}"})

        results = [None] * len(items)
        by_lang = {}
        for i, it in enumerate(items):
            lang = "ko" if it.get("lang") == "ko" else "en"
            text = str(it.get("text", ""))[:MAX_CHARS].strip()
            if text:
                by_lang.setdefault(lang, []).append((i, text))
        for lang, rows in by_lang.items():
            clf = CLASSIFIERS[lang]
            for (i, _), res in zip(rows, clf.classify([t for _, t in rows])):
                results[i] = {"id": items[i].get("id"), "model": clf.name, **res}
        return self._json(200, {"results": [r for r in results if r is not None]})

    def log_message(self, fmt, *args):  # 요청마다 줄을 찍지 않는다 — 배치가 몇 백 번 부른다.
        pass


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8200), Handler).serve_forever()
